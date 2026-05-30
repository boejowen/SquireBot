// Module augmentation for @tanstack/table-core's ColumnMeta so columnDefs can
// carry `meta: { filter: 'facet' | 'text' }` (the DataGrid reads it to pick a
// per-column filter control: a faceted <select> for Status/Tier/Class, a text
// input otherwise). Without this augmentation TS rejects the unknown meta key.
import '@tanstack/table-core';
import type { ColumnFilterMeta } from '$lib/columns';

declare module '@tanstack/table-core' {
	// eslint-disable-next-line @typescript-eslint/no-unused-vars
	interface ColumnMeta<TData extends RowData, TValue> extends ColumnFilterMeta {}
}
