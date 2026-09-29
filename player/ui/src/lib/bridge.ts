export const invoke = async <T,>(command: string, args?: Record<string, unknown>): Promise<T> => {
 const call = window.__TAURI__?.core?.invoke;
 if (!call) throw new Error('no tauri bridge');
 return call<T>(command,args);
};
export const listen = window.__TAURI__?.event?.listen ?? (async () => () => {});
export const getAppWindow = () => window.__TAURI__?.window?.getCurrentWindow?.();
