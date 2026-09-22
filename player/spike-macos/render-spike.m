// A throwaway spike, not a program anybody ships.
//
//   clang -fobjc-arc -framework Cocoa -framework OpenGL render-spike.m \
//     -L<the pinned lib/ directory> -lmpv -o render-spike
//   DYLD_LIBRARY_PATH=<the pinned lib/ directory> ./render-spike <a film> [seconds]
//
// **The question it exists to answer.** The pinned macOS engine
// (178meorg/libmpv-macos-build v0.41.4) is built with `vulkan=disabled`,
// `macos-cocoa-cb=disabled` and `swift-build=disabled`, so it has *no video
// output that can present into a window* - not into its own, not into ours. Its
// one video path is the libmpv render API over OpenGL (`plain-gl=enabled`), where
// the host owns the GL context and draws every frame. mpv 0.41 also dropped the
// `--wid` support that would have let us hand it a view, so this is the only way
// the film can appear inside a window Theia owns - which is what the OSD over the
// picture requires (design system 6b).
//
// So: three things to observe, and nothing else.
//   1. does the engine accept `vo=libmpv` and start playing at all?
//   2. does `mpv_render_context_render` into our own NSOpenGLView produce frames?
//   3. does VideoToolbox decode, and does CoreAudio open, with that VO?
//
// What it does NOT answer: whether the Tauri webview composites above this
// surface (the next spike), the OSD, tracks, subtitles, or anything about the
// real player's structure.

#import <Cocoa/Cocoa.h>
#import <OpenGL/gl3.h>
#import <dlfcn.h>
#import <mpv/client.h>
#import <mpv/render.h>
#import <mpv/render_gl.h>

static mpv_handle *gMpv;
static mpv_render_context *gRender;
static long gFrames;
static int gSeconds = 8;
static NSString *gMedia;

// mpv asks for every GL entry point it needs through this callback, and the
// answer has to be the *same* GL context's function - which on macOS is simply
// the process's, because OpenGL.framework exports them.
static void *get_proc_address(void *ctx, const char *name)
{
    (void)ctx;
    return dlsym(RTLD_DEFAULT, name);
}

// Called from mpv's own thread. Only a flag is touched: the render call itself
// must happen on the thread that owns the GL context.
static void on_mpv_render_update(void *ctx)
{
    (void)ctx;
}

@interface TheiaSpikeView : NSOpenGLView
@end

@implementation TheiaSpikeView

- (void)drawRect:(NSRect)dirtyRect
{
    (void)dirtyRect;
    if (!gRender) {
        return;
    }
    [[self openGLContext] makeCurrentContext];

    NSRect bounds = [self bounds];
    int width = (int)(bounds.size.width * 2.0);   // 2.0: this machine's Retina scale
    int height = (int)(bounds.size.height * 2.0);

    mpv_opengl_fbo fbo = {.fbo = 0, .w = width, .h = height, .internal_format = 0};
    int flip = 1;
    mpv_render_param params[] = {
        {MPV_RENDER_PARAM_OPENGL_FBO, &fbo},
        {MPV_RENDER_PARAM_FLIP_Y, &flip},
        {0},
    };
    int result = mpv_render_context_render(gRender, params);
    if (result < 0) {
        fprintf(stderr, "render failed: %s\n", mpv_error_string(result));
    } else {
        gFrames++;
    }
    [[self openGLContext] flushBuffer];
    mpv_render_context_report_swap(gRender);
}

@end

static void report(mpv_handle *mpv, double elapsed)
{
    const char *vo = mpv_get_property_string(mpv, "current-vo");
    const char *hwdec = mpv_get_property_string(mpv, "hwdec-current");
    const char *ao = mpv_get_property_string(mpv, "current-ao");
    double pos = 0.0;
    mpv_get_property(mpv, "time-pos", MPV_FORMAT_DOUBLE, &pos);
    printf("t=%4.1fs  frames=%ld  pos=%6.2f  vo=%s  hwdec=%s  ao=%s\n", elapsed, gFrames, pos,
           vo ? vo : "(none)", hwdec ? hwdec : "(none)", ao ? ao : "(none)");
    fflush(stdout);
    mpv_free((void *)vo);
    mpv_free((void *)hwdec);
    mpv_free((void *)ao);
}

int main(int argc, char *argv[])
{
    if (argc < 2) {
        fprintf(stderr, "usage: render-spike <a film> [seconds]\n");
        return 2;
    }
    gMedia = [NSString stringWithUTF8String:argv[1]];
    if (argc > 2) {
        gSeconds = atoi(argv[2]);
    }

    @autoreleasepool {
        [NSApplication sharedApplication];
        [NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];

        // A plain window with a GL view in it: the spike's subject is the render
        // path, not the window. The real player draws into a transparent Tauri
        // window with the OSD above it.
        NSRect frame = NSMakeRect(80, 80, 960, 540);
        NSWindow *window = [[NSWindow alloc] initWithContentRect:frame
                                                       styleMask:NSWindowStyleMaskTitled |
                                                                 NSWindowStyleMaskClosable |
                                                                 NSWindowStyleMaskResizable
                                                         backing:NSBackingStoreBuffered
                                                           defer:NO];
        [window setTitle:@"Theia render spike - vo=libmpv over our own GL context"];

        NSOpenGLPixelFormatAttribute attributes[] = {
            NSOpenGLPFAOpenGLProfile, NSOpenGLProfileVersion3_2Core,
            NSOpenGLPFADoubleBuffer,
            NSOpenGLPFAColorSize, 24,
            NSOpenGLPFAAlphaSize, 8,
            0,
        };
        NSOpenGLPixelFormat *format = [[NSOpenGLPixelFormat alloc] initWithAttributes:attributes];
        if (!format) {
            fprintf(stderr, "no OpenGL 3.2 core pixel format: this Mac cannot run the spike\n");
            return 1;
        }
        TheiaSpikeView *view = [[TheiaSpikeView alloc] initWithFrame:frame pixelFormat:format];
        [window setContentView:view];
        [window makeKeyAndOrderFront:nil];
        [NSApp activateIgnoringOtherApps:YES];

        // The engine, loaded from wherever DYLD_LIBRARY_PATH points.
        gMpv = mpv_create();
        if (!gMpv) {
            fprintf(stderr, "mpv_create failed\n");
            return 1;
        }
        // vo=libmpv is the whole point: mpv renders nothing itself, we do.
        mpv_set_option_string(gMpv, "vo", "libmpv");
        mpv_set_option_string(gMpv, "hwdec", "videotoolbox");
        mpv_set_option_string(gMpv, "ao", "coreaudio");
        mpv_set_option_string(gMpv, "mute", "yes");
        mpv_set_option_string(gMpv, "keep-open", "yes");
        mpv_set_option_string(gMpv, "terminal", "yes");
        mpv_set_option_string(gMpv, "msg-level", "all=warn,vo=v");

        int result = mpv_initialize(gMpv);
        if (result < 0) {
            fprintf(stderr, "mpv_initialize failed: %s\n", mpv_error_string(result));
            return 1;
        }
        printf("engine: %s\n", mpv_get_property_string(gMpv, "mpv-version"));
        printf("options accepted: vo=libmpv hwdec=videotoolbox ao=coreaudio\n");

        [[view openGLContext] makeCurrentContext];
        mpv_opengl_init_params gl_init = {.get_proc_address = get_proc_address,
                                          .get_proc_address_ctx = NULL};
        int advanced = 1;
        mpv_render_param create_params[] = {
            {MPV_RENDER_PARAM_API_TYPE, (void *)MPV_RENDER_API_TYPE_OPENGL},
            {MPV_RENDER_PARAM_OPENGL_INIT_PARAMS, &gl_init},
            {MPV_RENDER_PARAM_ADVANCED_CONTROL, &advanced},
            {0},
        };
        result = mpv_render_context_create(&gRender, gMpv, create_params);
        if (result < 0) {
            fprintf(stderr, "mpv_render_context_create failed: %s\n", mpv_error_string(result));
            fprintf(stderr, "  (this is the finding, if it is the finding: the engine has no GL render path)\n");
            return 1;
        }
        printf("render context created (OpenGL, advanced control)\n");
        mpv_render_context_set_update_callback(gRender, on_mpv_render_update, NULL);

        const char *command[] = {"loadfile", [gMedia UTF8String], NULL};
        mpv_command(gMpv, command);

        // A timer rather than a display link: the spike is about whether frames
        // arrive, not about pacing. mpv_render_context_update says when there is
        // one, and it is the documented way to ask.
        __block int elapsed = 0;
        NSTimer *tick = [NSTimer scheduledTimerWithTimeInterval:0.5 repeats:YES block:^(NSTimer *timer) {
            (void)timer;
            uint64_t flags = mpv_render_context_update(gRender);
            if (flags & MPV_RENDER_UPDATE_FRAME) {
                [view setNeedsDisplay:YES];
            }
            elapsed += 0.5;
            if (elapsed % 1 == 0) {
                report(gMpv, elapsed);
            }
            if (elapsed >= gSeconds) {
                [timer invalidate];
                printf("\n--- verdict ---\n");
                printf("frames rendered into our GL view: %ld\n", gFrames);
                printf("frames > 0 means the render API path works with this engine.\n");
                printf("Look at the window: a film visible in it, or black?\n");
                [NSApp terminate:nil];
            }
        }];
        (void)tick;
        [NSApp run];
    }
    return gFrames > 0 ? 0 : 1;
}
