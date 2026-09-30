#!/usr/bin/env python3
"""Exercise the installed player's pause, resize, fullscreen and resume on X11.

Run in the package proof's isolated X display, with its diagnostics directory.
Coordinates are translated from the actual window, never from desktop guesses.
"""
import ctypes as c, json, os, pathlib, re, subprocess, sys, time
p=pathlib.Path(sys.argv[1])
display=os.environ["DISPLAY"]
x=c.CDLL('libX11.so.6'); t=c.CDLL('libXtst.so.6')
x.XOpenDisplay.argtypes=[c.c_char_p]; x.XOpenDisplay.restype=c.c_void_p
d=x.XOpenDisplay(display.encode()); assert d
xid=int(re.search(r'window id (\d+)',(p/'player.log').read_text())[1])
x.XSetInputFocus.argtypes=[c.c_void_p,c.c_ulong,c.c_int,c.c_ulong]
x.XKeysymToKeycode.argtypes=[c.c_void_p,c.c_ulong]; x.XKeysymToKeycode.restype=c.c_uint
x.XStringToKeysym.argtypes=[c.c_char_p]; x.XStringToKeysym.restype=c.c_ulong
x.XFlush.argtypes=[c.c_void_p]
x.XGetInputFocus.argtypes=[c.c_void_p,c.POINTER(c.c_ulong),c.POINTER(c.c_int)]
x.XResizeWindow.argtypes=[c.c_void_p,c.c_ulong,c.c_uint,c.c_uint]
x.XDefaultRootWindow.argtypes=[c.c_void_p]; x.XDefaultRootWindow.restype=c.c_ulong
x.XTranslateCoordinates.argtypes=[c.c_void_p,c.c_ulong,c.c_ulong,c.c_int,c.c_int,c.POINTER(c.c_int),c.POINTER(c.c_int),c.POINTER(c.c_ulong)]
t.XTestFakeKeyEvent.argtypes=[c.c_void_p,c.c_uint,c.c_int,c.c_ulong]
t.XTestFakeMotionEvent.argtypes=[c.c_void_p,c.c_int,c.c_int,c.c_int,c.c_ulong]
t.XTestFakeButtonEvent.argtypes=[c.c_void_p,c.c_uint,c.c_int,c.c_ulong]
x.XSetInputFocus(d,xid,2,0)
def key(name):
    focus=c.c_ulong(); revert=c.c_int()
    x.XGetInputFocus(d,c.byref(focus),c.byref(revert))
    code=x.XKeysymToKeycode(d,x.XStringToKeysym(name))
    t.XTestFakeKeyEvent(d,code,1,0); t.XTestFakeKeyEvent(d,code,0,0); x.XFlush(d)
def status():
    return json.loads([s for s in (p/'player.log').read_text().splitlines() if s.startswith('{')][-1])
def report():
    for _ in range(10):
        try: return json.loads((p/'window-report.json').read_text())
        except json.JSONDecodeError: time.sleep(.05)
    raise AssertionError('no complete report')
def capture(name):
    r=report()['physical']; a=c.c_int(); b=c.c_int(); child=c.c_ulong()
    x.XTranslateCoordinates(d,xid,x.XDefaultRootWindow(d),0,0,c.byref(a),c.byref(b),c.byref(child))
    subprocess.run(['ffmpeg','-hide_banner','-loglevel','error','-y','-f','x11grab','-video_size',f"{r['width']}x{r['height']}",'-i',f"{display}+{a.value},{b.value}",'-frames:v','1',str(p/name)],check=True)
for _ in range(60):
    try:
        if report()['page']['bar']['controlsVisible'] > 0 and (status().get('osdFrames') or 0) > 0: break
    except (TypeError,KeyError): pass
    time.sleep(.25)
else: raise AssertionError('the interface has not booted')
time.sleep(.3)
key(b'k'); time.sleep(2); assert status()['pause'] is True,status()
paused=status()['pos']; time.sleep(1); assert abs(status()['pos']-paused)<.1
x.XResizeWindow(d,xid,960,540); x.XFlush(d); time.sleep(2)
assert report()['physical']=={'width':960,'height':540},report()
capture('paused-resize.png')
key(b'f'); time.sleep(2)
assert report()['physical']=={'width':1280,'height':720},report()
capture('paused-fullscreen.png')
key(b'f'); time.sleep(2)
assert report()['physical']=={'width':960,'height':540},report()
capture('paused-restored.png')
assert abs(status()['pos']-paused)<.1
# Click the observed play button after restoring the 960x540 window.
a=c.c_int(); b=c.c_int(); child=c.c_ulong()
x.XTranslateCoordinates(d,xid,x.XDefaultRootWindow(d),0,0,c.byref(a),c.byref(b),c.byref(child))
t.XTestFakeMotionEvent(d,-1,a.value+124,b.value+493,0)
t.XTestFakeButtonEvent(d,1,1,0); t.XTestFakeButtonEvent(d,1,0,0); x.XFlush(d)
for _ in range(24):
    time.sleep(.25)
    if status()['pause'] is False and status()['pos']>paused+.5: break
assert status()['pause'] is False and status()['pos']>paused+.5,status()
capture('resumed-window.png')
print('PASS paused resize, native fullscreen, restored window and resumed playback')
