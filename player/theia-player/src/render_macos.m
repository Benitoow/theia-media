// The native film surface under Tauri's transparent WKWebView. These mpv
// headers are extracted from the digest-checked engine archive at build time.
#import <AppKit/AppKit.h>
#import <OpenGL/gl3.h>
#import <dlfcn.h>
#import <mpv/client.h>
#import <mpv/render.h>
#import <mpv/render_gl.h>
#import <stdatomic.h>
#import <stdint.h>
#import <stdbool.h>
#import <stdio.h>

typedef int (*RenderCreate)(mpv_render_context **, mpv_handle *, mpv_render_param *);
typedef void (*RenderCallback)(mpv_render_context *, mpv_render_update_fn, void *);
typedef uint64_t (*RenderUpdate)(mpv_render_context *);
typedef int (*RenderFrame)(mpv_render_context *, mpv_render_param *);
typedef void (*RenderSwap)(mpv_render_context *);
typedef void (*RenderFree)(mpv_render_context *);

static void *gLibrary;
static mpv_render_context *gRender;
static RenderCreate gCreate;
static RenderCallback gCallback;
static RenderUpdate gUpdate;
static RenderFrame gFrame;
static RenderSwap gSwap;
static RenderFree gFree;
static atomic_uint_fast64_t gFrames;
static atomic_uint_fast64_t gNotifications;
// Why a picture is missing cannot be answered by a frame count, and the first
// Mac run that had none could only say "1 -> 1". Each link of the chain is
// counted where it happens - the tick that asks, the frame mpv says is ready,
// the draw AppKit performs, the render that fails - and whether the window was
// on screen while it did. `theia_render_diagnostics` prints them as one line.
static atomic_uint_fast64_t gTicks;
static atomic_uint_fast64_t gReady;
static atomic_uint_fast64_t gDraws;
static atomic_uint_fast64_t gErrors;
static atomic_uint_fast64_t gOnScreen;
static atomic_uint_fast64_t gSurface;
static atomic_bool gAttached;
// When, not just how many. `ticks=12` in a run that lasted seconds says the
// timer stopped, and this says *when* it stopped and whether the thread that
// stopped it was inside `mpv_render_context_render` when it did - which is the
// difference between "AppKit asked for no more draws" and "the render call
// never returned". Milliseconds since the surface was attached.
static uint64_t gAttachedAtMs;
static atomic_uint_fast64_t gLastTickMs;
static atomic_uint_fast64_t gLastDrawMs;
static atomic_uint_fast64_t gInRender;
// When the render call that is in flight was entered. `in-render=1` says a call
// has not returned; this says whether it has been inside for 40 ms or for six
// seconds, which is the difference between a slow machine and a hang.
static atomic_uint_fast64_t gLastRenderMs;
// Which renderer answered. A frame rate means nothing without it: the same code
// draws at the display's pace on a GPU and at a crawl under Apple's software
// renderer, and the proof runner is a virtual machine with no GPU at all.
// Written once, at attach, from the context that is current then.
static char gRenderer[128];

static uint64_t now_ms(void)
{
    return (uint64_t)(CFAbsoluteTimeGetCurrent() * 1000.0);
}

@interface TheiaFilmView : NSOpenGLView
@end

static TheiaFilmView *gView;
static NSTimer *gTimer;

static void *get_proc_address(void *context, const char *name)
{
    (void)context;
    return dlsym(RTLD_DEFAULT, name);
}

// mpv calls this from its own thread. AppKit and OpenGL stay on the main thread.
static void on_render_update(void *context)
{
    (void)context;
    atomic_fetch_add_explicit(&gNotifications, 1, memory_order_relaxed);
}

@implementation TheiaFilmView

- (void)drawRect:(NSRect)dirty
{
    (void)dirty;
    atomic_fetch_add_explicit(&gDraws, 1, memory_order_relaxed);
    atomic_store_explicit(&gLastDrawMs, now_ms() - gAttachedAtMs, memory_order_relaxed);
    if (!gRender) return;
    [[self openGLContext] makeCurrentContext];
    [[self openGLContext] update];
    NSRect backing = [self convertRectToBacking:self.bounds];
    atomic_store_explicit(
        &gSurface,
        ((uint64_t)(uint32_t)backing.size.width << 32) | (uint32_t)backing.size.height,
        memory_order_relaxed);
    mpv_opengl_fbo fbo = {
        .fbo = 0,
        .w = (int)backing.size.width,
        .h = (int)backing.size.height,
        .internal_format = 0,
    };
    if (fbo.w <= 0 || fbo.h <= 0) return;
    int flip = 1;
    mpv_render_param params[] = {
        {MPV_RENDER_PARAM_OPENGL_FBO, &fbo},
        {MPV_RENDER_PARAM_FLIP_Y, &flip},
        {0},
    };
    atomic_fetch_add_explicit(&gInRender, 1, memory_order_relaxed);
    atomic_store_explicit(&gLastRenderMs, now_ms() - gAttachedAtMs, memory_order_relaxed);
    int result = gFrame(gRender, params);
    if (result < 0) {
        atomic_fetch_sub_explicit(&gInRender, 1, memory_order_relaxed);
        atomic_fetch_add_explicit(&gErrors, 1, memory_order_relaxed);
        fprintf(stderr, "theia-player: macOS frame render failed (%d)\n", result);
        return;
    }
    atomic_fetch_sub_explicit(&gInRender, 1, memory_order_relaxed);
    [[self openGLContext] flushBuffer];
    gSwap(gRender);
    atomic_fetch_add_explicit(&gFrames, 1, memory_order_relaxed);
}

@end

void theia_render_detach(void)
{
    atomic_store_explicit(&gAttached, false, memory_order_relaxed);
    if (gTimer) {
        [gTimer invalidate];
        gTimer = nil;
    }
    if (gRender) {
        [[gView openGLContext] makeCurrentContext];
        gCallback(gRender, NULL, NULL);
        gFree(gRender);
        gRender = NULL;
    }
    [gView removeFromSuperview];
    gView = nil;
    if (gLibrary) {
        dlclose(gLibrary);
        gLibrary = NULL;
    }
}

static bool fail(char *error, size_t capacity, const char *message)
{
    if (capacity > 0) snprintf(error, capacity, "%s", message);
    theia_render_detach();
    return false;
}

bool theia_render_attach(void *content_view_pointer, void *mpv_pointer,
                         const char *library_path, char *error, size_t capacity)
{
    if (![NSThread isMainThread]) return fail(error, capacity, "macOS rendering must start on the main thread");
    theia_render_detach();
    // Tauri's ns_view() is the window content view. Wry installs its WKWebView
    // as a child of that view, so looking for a parent of ns_view() would find
    // nothing and make every Mac player fail before the first frame.
    NSView *parent = (__bridge NSView *)content_view_pointer;
    NSView *webview = nil;
    Class webview_class = NSClassFromString(@"WKWebView");
    for (NSView *child in parent.subviews) {
        if ([child isKindOfClass:webview_class]) {
            webview = child;
            break;
        }
    }
    if (!parent || !webview || !mpv_pointer) return fail(error, capacity, "the window has no WKWebView or engine");

    gLibrary = dlopen(library_path, RTLD_NOW | RTLD_GLOBAL);
    if (!gLibrary) return fail(error, capacity, dlerror());
    gCreate = (RenderCreate)dlsym(gLibrary, "mpv_render_context_create");
    gCallback = (RenderCallback)dlsym(gLibrary, "mpv_render_context_set_update_callback");
    gUpdate = (RenderUpdate)dlsym(gLibrary, "mpv_render_context_update");
    gFrame = (RenderFrame)dlsym(gLibrary, "mpv_render_context_render");
    gSwap = (RenderSwap)dlsym(gLibrary, "mpv_render_context_report_swap");
    gFree = (RenderFree)dlsym(gLibrary, "mpv_render_context_free");
    if (!gCreate || !gCallback || !gUpdate || !gFrame || !gSwap || !gFree) {
        return fail(error, capacity, "the pinned libmpv has no complete render API");
    }

    NSOpenGLPixelFormatAttribute attributes[] = {
        NSOpenGLPFAOpenGLProfile, NSOpenGLProfileVersion3_2Core,
        NSOpenGLPFADoubleBuffer,
        NSOpenGLPFAColorSize, 24,
        NSOpenGLPFAAlphaSize, 8,
        0,
    };
    NSOpenGLPixelFormat *format = [[NSOpenGLPixelFormat alloc] initWithAttributes:attributes];
    if (!format) return fail(error, capacity, "macOS has no OpenGL 3.2 core pixel format");
    gView = [[TheiaFilmView alloc] initWithFrame:webview.frame pixelFormat:format];
    if (!gView) return fail(error, capacity, "macOS could not create the film surface");
    gView.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
    [gView setWantsBestResolutionOpenGLSurface:YES];
    [parent addSubview:gView positioned:NSWindowBelow relativeTo:webview];
    [[gView openGLContext] makeCurrentContext];
    const GLubyte *renderer = glGetString(GL_RENDERER);
    snprintf(gRenderer, sizeof(gRenderer), "%s", renderer ? (const char *)renderer : "unknown");

    mpv_opengl_init_params gl = {
        .get_proc_address = get_proc_address,
        .get_proc_address_ctx = NULL,
    };
    // No `MPV_RENDER_PARAM_ADVANCED_CONTROL`, and that is a correction rather
    // than a preference. The header's Threading section says the thread calling
    // `mpv_render_*` "does not call libmpv API functions other than the
    // mpv_render_* functions", that "there must be no lock or wait dependency
    // from the render thread to a thread using other libmpv functions", and:
    // "If you set MPV_RENDER_PARAM_ADVANCED_CONTROL, you promise that this won't
    // happen, and must absolutely guarantee it, or a real deadlock will freeze
    // the mpv core thread forever."
    //
    // This bridge breaks exactly that rule: it renders on the application's main
    // thread, which is also the thread calling `loadfile`, `set_option` and every
    // status read. Measured on 28 September 2026 (run of commit `49917a6`, the
    // `macos-intel` job): one frame reached the screen, the second draw never
    // returned, and the timer stopped - `ticks=12` in a run of several seconds.
    // Without advanced control, mpv does not wait for the host's handshake, so
    // the documented failure degrades to a timeout instead of a freeze; the
    // header notes the cost as "playback quality will be degraded", and that
    // cost is accepted here only until rendering moves to its own thread, which
    // is what this API recommends and what this file still owes.
    int advanced = 0;
    mpv_render_param params[] = {
        {MPV_RENDER_PARAM_API_TYPE, (void *)MPV_RENDER_API_TYPE_OPENGL},
        {MPV_RENDER_PARAM_OPENGL_INIT_PARAMS, &gl},
        {MPV_RENDER_PARAM_ADVANCED_CONTROL, &advanced},
        {0},
    };
    int result = gCreate(&gRender, (mpv_handle *)mpv_pointer, params);
    if (result < 0) {
        char detail[120];
        snprintf(detail, sizeof(detail), "mpv_render_context_create failed (%d)", result);
        return fail(error, capacity, detail);
    }
    atomic_store_explicit(&gFrames, 0, memory_order_relaxed);
    atomic_store_explicit(&gNotifications, 0, memory_order_relaxed);
    atomic_store_explicit(&gTicks, 0, memory_order_relaxed);
    atomic_store_explicit(&gReady, 0, memory_order_relaxed);
    atomic_store_explicit(&gDraws, 0, memory_order_relaxed);
    atomic_store_explicit(&gErrors, 0, memory_order_relaxed);
    atomic_store_explicit(&gOnScreen, 0, memory_order_relaxed);
    atomic_store_explicit(&gSurface, 0, memory_order_relaxed);
    atomic_store_explicit(&gInRender, 0, memory_order_relaxed);
    atomic_store_explicit(&gLastTickMs, 0, memory_order_relaxed);
    atomic_store_explicit(&gLastDrawMs, 0, memory_order_relaxed);
    atomic_store_explicit(&gLastRenderMs, 0, memory_order_relaxed);
    gAttachedAtMs = now_ms();
    gCallback(gRender, on_render_update, NULL);

    // `update()` is what asks mpv whether there is a frame to draw, and it is
    // still the right thing to call on every tick: the header says the user "is
    // supposed to call this when the update callback was invoked (like all
    // mpv_render_* functions, this has to happen on the render thread, and _not_
    // from the update callback itself)". The timer runs in common modes so
    // dragging a window cannot starve rendering.
    gTimer = [NSTimer timerWithTimeInterval:1.0 / 60.0 repeats:YES block:^(NSTimer *timer) {
        (void)timer;
        if (!gRender) return;
        atomic_fetch_add_explicit(&gTicks, 1, memory_order_relaxed);
        atomic_store_explicit(&gLastTickMs, now_ms() - gAttachedAtMs, memory_order_relaxed);
        // Sampled here, on the main thread, because AppKit answers for its own
        // thread: a window that is hidden or fully occluded is never asked to
        // draw, and "no frames" would then be a fact about the window and not
        // about the renderer.
        NSWindow *window = [gView window];
        bool onScreen = window != nil && window.isVisible &&
            (window.occlusionState & NSWindowOcclusionStateVisible) != 0;
        atomic_store_explicit(&gOnScreen, onScreen ? 1 : 0, memory_order_relaxed);
        [[gView openGLContext] makeCurrentContext];
        if (gUpdate(gRender) & MPV_RENDER_UPDATE_FRAME) {
            atomic_fetch_add_explicit(&gReady, 1, memory_order_relaxed);
            [gView setNeedsDisplay:YES];
        }
    }];
    [[NSRunLoop mainRunLoop] addTimer:gTimer forMode:NSRunLoopCommonModes];
    atomic_store_explicit(&gAttached, true, memory_order_relaxed);
    return true;
}

uint64_t theia_render_frames(void)
{
    return atomic_load_explicit(&gFrames, memory_order_relaxed);
}

// The window server's own id for a window: what `screencapture -l` wants when a
// proof run has to photograph the film's window rather than the screen it may
// not be on. Tauri hands over its NSWindow pointer and nothing else about it is
// touched. Returns 0 when there is no window to name.
uint64_t theia_window_number(void *window_pointer)
{
    if (!window_pointer) return 0;
    return (uint64_t)[(__bridge NSWindow *)window_pointer windowNumber];
}

// Why a picture is missing, in one line a proof log can lift with a single grep.
// `attached=0` with everything else at zero is a surface that was never made;
// `ticks` without `ready` is mpv offering no frame; `ready` without `draws` is
// AppKit never asking the view to draw - and `on-screen=0` while that happens
// says the window, not the renderer, is the answer. The frame count alone said
// "1 -> 1" and none of that.
//
// `notifications` counts mpv's own update callbacks and is monotonic - it is
// reset when a surface is attached, not by the tick that reads it, so the only
// thing it has to say is whether it rises while a film plays. A tick that
// cleared it would have made "mpv never called us" and "mpv called and the
// reader was a second late" the same number.
// `surface` is the backing store the last draw asked for, printed as WxH, and
// stays 0x0 until one happens.
void theia_render_diagnostics(char *out, size_t capacity)
{
    if (capacity == 0) return;
    uint64_t surface = atomic_load_explicit(&gSurface, memory_order_relaxed);
    snprintf(out, capacity,
             "attached=%d elapsed-ms=%llu ticks=%llu last-tick-ms=%llu ready=%llu draws=%llu "
             "last-draw-ms=%llu in-render=%llu last-render-ms=%llu errors=%llu frames=%llu "
             "notifications=%llu on-screen=%llu surface=%ux%u gl-renderer=\"%s\"",
             atomic_load_explicit(&gAttached, memory_order_relaxed) ? 1 : 0,
             (unsigned long long)(now_ms() - gAttachedAtMs),
             (unsigned long long)atomic_load_explicit(&gTicks, memory_order_relaxed),
             (unsigned long long)atomic_load_explicit(&gLastTickMs, memory_order_relaxed),
             (unsigned long long)atomic_load_explicit(&gReady, memory_order_relaxed),
             (unsigned long long)atomic_load_explicit(&gDraws, memory_order_relaxed),
             (unsigned long long)atomic_load_explicit(&gLastDrawMs, memory_order_relaxed),
             (unsigned long long)atomic_load_explicit(&gInRender, memory_order_relaxed),
             (unsigned long long)atomic_load_explicit(&gLastRenderMs, memory_order_relaxed),
             (unsigned long long)atomic_load_explicit(&gErrors, memory_order_relaxed),
             (unsigned long long)atomic_load_explicit(&gFrames, memory_order_relaxed),
             (unsigned long long)atomic_load_explicit(&gNotifications, memory_order_relaxed),
             (unsigned long long)atomic_load_explicit(&gOnScreen, memory_order_relaxed),
             (unsigned)(surface >> 32), (unsigned)(surface & 0xffffffffu),
             gRenderer);
}
