#ifndef GO_MDBTOOLS_BRIDGE_H
#define GO_MDBTOOLS_BRIDGE_H

#include <mdbtools.h>

typedef struct {
    MdbHandle *mdb;
    MdbTableDef *table;
    char **values;
    int *lengths;
    unsigned char *nulls;
} GoMdbReader;

GoMdbReader *go_mdb_open_reader(const char *path, const char *table_name);
void go_mdb_close_reader(GoMdbReader *reader);
int go_mdb_next(GoMdbReader *reader);
MdbColumn *go_mdb_column(GoMdbReader *reader, int index);
MdbCatalogEntry *go_mdb_entry(GPtrArray *catalog, unsigned int index);
int go_mdb_is_null(GoMdbReader *reader, int index);
int go_mdb_memo_too_large(GoMdbReader *reader, int index);
int go_mdb_length(GoMdbReader *reader, int index);
const char *go_mdb_text(GoMdbReader *reader, int index);
void *go_mdb_blob(GoMdbReader *reader, int index, size_t *size);

#endif