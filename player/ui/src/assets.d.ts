// Ambient asset modules. This file must stay a script (no top-level
// import/export): with moduleResolution "bundler", wildcard declarations in
// a module .d.ts are silently ignored and every asset import fails TS2307.
declare module '*.png' {
	const src: string;
	export default src;
}
declare module '*.gif' {
	const src: string;
	export default src;
}
declare module '*.jpg' {
	const src: string;
	export default src;
}
