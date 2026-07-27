// Theia's frontend is a pure single-page app. There is no Node server in
// production -- just a Go binary serving static files -- so nothing is rendered
// or prerendered ahead of time.
export const ssr = false;
export const prerender = false;
