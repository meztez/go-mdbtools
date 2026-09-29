#include "bridge.h"

#define GO_MDB_BIND_SIZE 65536
#define GO_MDB_BLOB_BUFFER (MDB_BIND_SIZE * 64)
#define GO_MDB_BLOB_LIMIT (32u * 1024u * 1024u)

GoMdbReader *go_mdb_open_reader(const char *path, const char *table_name) {
    GoMdbReader *reader = calloc(1, sizeof(*reader));
    if (!reader) return NULL;
    reader->mdb = mdb_open(path, MDB_NOFLAGS);
    if (!reader->mdb) goto fail;
    reader->table = mdb_read_table_by_name(reader->mdb, (char *)table_name, MDB_TABLE);
    if (!reader->table || !mdb_read_columns(reader->table)) goto fail;
    mdb_set_bind_size(reader->mdb, GO_MDB_BIND_SIZE);
    mdb_set_boolean_fmt_numbers(reader->mdb);
    mdb_set_date_fmt(reader->mdb, "%Y-%m-%d %H:%M:%S");
    mdb_set_shortdate_fmt(reader->mdb, "%Y-%m-%d %H:%M:%S");
    unsigned int count = reader->table->num_cols;
    reader->values = calloc(count, sizeof(*reader->values));
    reader->lengths = calloc(count, sizeof(*reader->lengths));
    reader->nulls = calloc(count, sizeof(*reader->nulls));
    if (!reader->values || !reader->lengths || !reader->nulls) goto fail;
    for (unsigned int index = 0; index < count; index++) {
        MdbColumn *column = g_ptr_array_index(reader->table->columns, index);
        reader->values[index] = calloc(column->col_type == MDB_OLE ? GO_MDB_BLOB_BUFFER : GO_MDB_BIND_SIZE, 1);
        if (!reader->values[index] || mdb_bind_column(reader->table, index + 1,
                reader->values[index], &reader->lengths[index]) < 0) goto fail;
    }
    mdb_rewind_table(reader->table);
    return reader;
fail:
    go_mdb_close_reader(reader);
    return NULL;
}

void go_mdb_close_reader(GoMdbReader *reader) {
    if (!reader) return;
    if (reader->values) {
        unsigned int count = reader->table ? reader->table->num_cols : 0;
        for (unsigned int index = 0; index < count; index++) free(reader->values[index]);
    }
    free(reader->values);
    free(reader->lengths);
    free(reader->nulls);
    if (reader->table) mdb_free_tabledef(reader->table);
    if (reader->mdb) mdb_close(reader->mdb);
    free(reader);
}

MdbColumn *go_mdb_column(GoMdbReader *reader, int index) {
    return g_ptr_array_index(reader->table->columns, index);
}

MdbCatalogEntry *go_mdb_entry(GPtrArray *catalog, unsigned int index) {
    return g_ptr_array_index(catalog, index);
}

int go_mdb_is_null(GoMdbReader *reader, int index) { return reader->nulls[index]; }
int go_mdb_memo_too_large(GoMdbReader *reader, int index) {
    MdbColumn *col = go_mdb_column(reader, index);
    return col->col_type == MDB_MEMO && col->cur_value_len >= MDB_MEMO_OVERHEAD &&
        (mdb_get_int32(reader->mdb->pg_buf, col->cur_value_start) & 0x3fffffff) > GO_MDB_BIND_SIZE / 2;
}
int go_mdb_length(GoMdbReader *reader, int index) { return reader->lengths[index]; }
const char *go_mdb_text(GoMdbReader *reader, int index) { return reader->values[index]; }

int go_mdb_next(GoMdbReader *reader) {
    if (!mdb_fetch_row(reader->table)) return 0;
    MdbHandle *mdb = reader->mdb;
    int start;
    size_t size;
    if (mdb_find_row(mdb, reader->table->cur_row - 1, &start, &size) < 0) return -1;
    start &= 0x1fff;
    MdbField *fields = calloc(reader->table->num_cols, sizeof(*fields));
    if (!fields) return -1;
    int count = mdb_crack_row(reader->table, start, size, fields);
    if (count < 0) {
        free(fields);
        return -1;
    }
    for (unsigned int index = 0; index < reader->table->num_cols; index++) {
        reader->nulls[fields[index].colnum] = fields[index].is_null;
    }
    free(fields);
    return 1;
}

void *go_mdb_blob(GoMdbReader *reader, int index, size_t *size) {
    MdbColumn *col = go_mdb_column(reader, index);
    if (col->col_type == MDB_OLE) {
        if (col->cur_value_len < MDB_MEMO_OVERHEAD ||
                (mdb_get_int32(col->bind_ptr, 0) & 0x3fffffff) > GO_MDB_BLOB_LIMIT) return NULL;
        return mdb_ole_read_full(reader->mdb, col, size);
    }
    *size = col->cur_value_len;
    if (*size == 0) return calloc(1, 1);
    void *value = malloc(*size);
    if (value) memcpy(value, reader->mdb->pg_buf + col->cur_value_start, *size);
    return value;
}