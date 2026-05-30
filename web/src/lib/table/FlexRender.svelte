<script lang="ts" module>
	// FlexRender — renders a TanStack columnDef `header`/`cell` value that may be
	// a primitive (string/number), a Svelte component, or a function returning
	// one, given the TanStack cell/header `context`. The local Svelte-5 analog of
	// @tanstack/*-table's flexRender (14-RESEARCH Pattern 3). Used for BOTH header
	// cells and body cells in DataGrid.
	//
	// A columnDef `cell`/`header` in this app is either:
	//   - a string label (header text), or
	//   - a function `(ctx) => primitive` (e.g. cell: (c) => c.getValue()), or
	//   - a function `(ctx) => ({ component, props })` to mount a Svelte
	//     component (StatusCell / a wiki link / the Item tooltip trigger). We use
	//     the `{ component, props }` shape (a `RenderComponentConfig`) so the
	//     columnDef stays plain data and the actual <Component> mount happens
	//     here, inside a reactive Svelte context.

	import type { Component } from 'svelte';

	/** Returned by a columnDef cell/header fn to mount a Svelte component. */
	export class RenderComponentConfig<TProps extends Record<string, unknown>> {
		component: Component<TProps>;
		props: TProps;
		constructor(component: Component<TProps>, props: TProps = {} as TProps) {
			this.component = component;
			this.props = props;
		}
	}

	/** Helper: wrap a component + props as a columnDef cell/header value. */
	export function renderComponent<TProps extends Record<string, unknown>>(
		component: Component<TProps>,
		props: TProps = {} as TProps
	): RenderComponentConfig<TProps> {
		return new RenderComponentConfig(component, props);
	}

	/** A columnDef cell/header may be: a primitive, a render config, or a function
	 * returning either, given the TanStack header/cell context. Typed loosely
	 * (the function arg is the TanStack context, whose concrete type differs for
	 * headers vs cells) so it accepts a `ColumnDefTemplate` directly. */
	export type FlexContent =
		| string
		| number
		| null
		| undefined
		// eslint-disable-next-line @typescript-eslint/no-explicit-any
		| RenderComponentConfig<any>
		// eslint-disable-next-line @typescript-eslint/no-explicit-any
		| ((context: any) => any);
</script>

<script lang="ts">
	let { content, context }: { content: FlexContent; context: unknown } = $props();

	// Resolve a function content against the TanStack context (header/cell ctx).
	let resolved = $derived(typeof content === 'function' ? content(context) : content);
</script>

{#if resolved == null}
	<!-- nothing to render -->
{:else if resolved instanceof RenderComponentConfig}
	<resolved.component {...resolved.props} />
{:else}
	{resolved}
{/if}
