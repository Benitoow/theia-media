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
    if (!gRender) return;
    [[self openGLContext] makeCurrentContext];
    [[self openGLContext] update];
    NSRect backing = [self convertRectToBacking:self.bounds];
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
    int result = gFrame(gRender, params);
    if (result < 0) {
        fprintf(stderr, "theia-player: macOS frame render failed (%d)\n", result);
        return;
    }
    [[self openGLContext] flushBuffer];
    gSwap(gRender);
    atomic_fetch_add_explicit(&gFrames, 1, memory_order_relaxed);
}

@end

void theia_render_detach(void)
{
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

    mpv_opengl_init_params gl = {
        .get_proc_address = get_proc_address,
        .get_proc_address_ctx = NULL,
    };
    int advanced = 1;
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
    gCallback(gRender, on_render_update, NULL);

    // Advanced control requires update() after callback notifications. The
    // timer runs in common modes so dragging a window cannot starve rendering.
    gTimer = [NSTimer timerWithTimeInterval:1.0 / 60.0 repeats:YES block:^(NSTimer *timer) {
        (void)timer;
        if (!gRender) return;
        [[gView openGLContext] makeCurrentContext];
        atomic_exchange_explicit(&gNotifications, 0, memory_order_relaxed);
        if (gUpdate(gRender) & MPV_RENDER_UPDATE_FRAME) [gView setNeedsDisplay:YES];
    }];
    [[NSRunLoop mainRunLoop] addTimer:gTimer forMode:NSRunLoopCommonModes];
    return true;
}

uint64_t theia_render_frames(void)
{
    return atomic_load_explicit(&gFrames, memory_order_relaxed);
}
