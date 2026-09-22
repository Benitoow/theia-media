// A throwaway spike, the sibling of render-spike.m, and the second half of the
// same question.
//
//   clang -fobjc-arc -framework Cocoa -framework OpenGL -framework WebKit \
//     composite-spike.m -o composite-spike
//   ./composite-spike            # looks at itself for ten seconds, then says what it saw
//
// **The question.** The macOS player cannot hand mpv a view any more (`--wid` is
// gone in 0.41) and the pinned engine has no window-presenting video output, so
// the film will be drawn by Theia into a GL surface it owns. The OSD - a web page
// - then has to sit *above* that surface, transparently, and still receive the
// clicks a viewer makes on the picture. On Windows this was spike A2b: mpv's child
// window sat at the bottom of the child z-order and the WebView2 stack above it.
// On macOS the mechanism is different (an NSView hierarchy with an explicit
// order) and nothing here has ever been run, so it gets its own spike rather than
// an assumption.
//
// What it answers, and nothing else:
//   1. does a transparent WKWebView composite above an NSOpenGLView in one window?
//   2. does the page still receive clicks through its own transparent areas?
//   3. does the GL surface keep drawing underneath, at the same time?
//
// What it does NOT answer: anything about mpv, the OSD's real markup, or the
// player's structure. It draws a moving colour instead of a film on purpose - the
// two spikes stay independent so that one failing does not hide the other.

#import <Cocoa/Cocoa.h>
#import <OpenGL/gl3.h>
#import <WebKit/WebKit.h>
#import <signal.h>
#import <stdarg.h>
#import <stdlib.h>
#import <unistd.h>

static long gSwaps;

// Same discipline as the render spike: nothing silent, nothing unbounded. The
// first version of that one hung on a headless runner with its output still in a
// buffer, and this one is asked the same kind of question on the same kind of
// machine.
static void step(const char *format, ...)
{
    va_list args;
    va_start(args, format);
    printf("step: ");
    vprintf(format, args);
    printf("\n");
    va_end(args);
    fflush(stdout);
}

static void watchdog(int signal)
{
    (void)signal;
    printf("\n--- verdict (watchdog) ---\n");
    printf("sixty seconds without a verdict; GL swaps so far: %ld\n", gSwaps);
    printf("if the last step above is the window or the run loop, this machine has no\n");
    printf("window session: the composite question needs a person with a Mac.\n");
    _exit(3);
}

// The "film": a colour that moves, so a photograph shows whether the surface
// underneath is still being drawn while the page is up.
@interface MovingSurface : NSOpenGLView
@end

@implementation MovingSurface

- (void)drawRect:(NSRect)dirtyRect
{
    (void)dirtyRect;
    [[self openGLContext] makeCurrentContext];
    double t = [NSDate timeIntervalSinceReferenceDate];
    glViewport(0, 0, (GLsizei)([self bounds].size.width * 2), (GLsizei)([self bounds].size.height * 2));
    glClearColor((GLfloat)(0.5 + 0.5 * sin(t)), (GLfloat)(0.2 + 0.4 * sin(t * 0.7)),
                 (GLfloat)(0.35 + 0.3 * cos(t * 0.4)), 1.0f);
    glClear(GL_COLOR_BUFFER_BIT);
    [[self openGLContext] flushBuffer];
    gSwaps++;
}

@end

@interface Compositor : NSObject <WKNavigationDelegate>
@property(nonatomic, strong) NSWindow *window;
@property(nonatomic, strong) MovingSurface *surface;
@property(nonatomic, strong) WKWebView *osd;
@end

@implementation Compositor

- (void)build
{
    NSRect frame = NSMakeRect(120, 120, 960, 540);
    self.window = [[NSWindow alloc] initWithContentRect:frame
                                             styleMask:NSWindowStyleMaskTitled |
                                                       NSWindowStyleMaskClosable |
                                                       NSWindowStyleMaskResizable
                                               backing:NSBackingStoreBuffered
                                                 defer:NO];
    [self.window setTitle:@"Theia composite spike - a transparent page over a GL surface"];

    // The content view is transparent so the window's own background cannot be
    // mistaken for either layer.
    NSView *content = [[NSView alloc] initWithFrame:frame];
    [self.window setContentView:content];
    [self.window setOpaque:NO];
    [self.window setBackgroundColor:[NSColor clearColor]];

    // 1. the film's surface, at the bottom.
    NSOpenGLPixelFormatAttribute attributes[] = {
        NSOpenGLPFAOpenGLProfile, NSOpenGLProfileVersion3_2Core,
        NSOpenGLPFADoubleBuffer,
        NSOpenGLPFAColorSize, 24,
        NSOpenGLPFAAlphaSize, 8,
        0,
    };
    NSOpenGLPixelFormat *format = [[NSOpenGLPixelFormat alloc] initWithAttributes:attributes];
    self.surface = [[MovingSurface alloc] initWithFrame:frame pixelFormat:format];
    self.surface.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
    [content addSubview:self.surface positioned:NSWindowBelow relativeTo:nil];

    // 2. the OSD, above it, transparent. underPageBackgroundColor is the
    // documented way to make a WKWebView not paint a background (macOS 12+);
    // drawsBackground is the older switch and is set as well because a page that
    // paints white here would make this spike answer the wrong question.
    WKWebViewConfiguration *configuration = [[WKWebViewConfiguration alloc] init];
    self.osd = [[WKWebView alloc] initWithFrame:frame configuration:configuration];
    self.osd.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
    if ([self.osd respondsToSelector:@selector(setUnderPageBackgroundColor:)]) {
        [self.osd setUnderPageBackgroundColor:[NSColor clearColor]];
    }
    @try {
        [self.osd setValue:@NO forKey:@"drawsBackground"];
    } @catch (NSException *e) {
        NSLog(@"drawsBackground is not settable here: %@", e.reason);
    }
    self.osd.navigationDelegate = self;
    [content addSubview:self.osd positioned:NSWindowAbove relativeTo:nil];

    NSString *page = @"<html><head><meta name=\"viewport\" content=\"width=device-width\">"
                      "<style>"
                      "html,body{margin:0;height:100%;background:transparent;"
                      "font:16px -apple-system,system-ui;color:#f4efe6}"
                      ".bar{position:fixed;left:0;right:0;bottom:0;height:64px;"
                      "background:rgba(12,10,9,0.55);backdrop-filter:blur(8px);"
                      "display:flex;align-items:center;gap:16px;padding:0 20px}"
                      ".pill{border:1px solid rgba(244,239,230,0.35);border-radius:999px;padding:8px 14px}"
                      ".title{position:fixed;left:24px;top:20px;font-size:28px;letter-spacing:0.02em;"
                      "text-shadow:0 1px 8px rgba(0,0,0,0.6)}"
                      "#clicks{position:fixed;right:24px;top:20px;opacity:0.8}"
                      "</style></head><body>"
                      "<div class=\"title\">THEIA - a page over the film</div>"
                      "<div id=\"clicks\">clicks: 0</div>"
                      "<div class=\"bar\"><span class=\"pill\">&#9654;</span>"
                      "<span class=\"pill\">0:12</span><span class=\"pill\">&#8942;</span>"
                      "<span class=\"pill\">&#9974;</span></div>"
                      "<script>let n=0;document.body.addEventListener('click',e=>{"
                      "n++;document.getElementById('clicks').textContent='clicks: '+n+"
                      "' at '+Math.round(e.clientX)+','+Math.round(e.clientY);});</script>"
                      "</body></html>";
    [self.osd loadHTMLString:page baseURL:nil];
}

- (void)webView:(WKWebView *)webView didFinishNavigation:(WKNavigation *)navigation
{
    (void)webView;
    (void)navigation;
    // A page that is opaque would cover the film; this asks the engine what it
    // thinks rather than trusting the configuration above.
    [self.osd evaluateJavaScript:@"getComputedStyle(document.body).backgroundColor"
               completionHandler:^(id result, NSError *error) {
                   NSLog(@"body background: %@ (error: %@)", result, error);
               }];
    [self.osd evaluateJavaScript:@"[window.innerWidth, window.innerHeight].join('x')"
               completionHandler:^(id result, NSError *error) {
                   NSLog(@"page viewport: %@ (error: %@)", result, error);
               }];
}

@end

int main(void)
{
    setvbuf(stdout, NULL, _IONBF, 0);
    signal(SIGALRM, watchdog);
    alarm(60);

    @autoreleasepool {
        step("asking AppKit for an application");
        [NSApplication sharedApplication];
        [NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];

        Compositor *compositor = [[Compositor alloc] init];
        step("building the window: a GL surface below, a transparent page above");
        [compositor build];
        [compositor.window makeKeyAndOrderFront:nil];
        [NSApp activateIgnoringOtherApps:YES];
        step("window created; on screen: %s", [compositor.window isVisible] ? "yes" : "NO (no window session)");

        __block double elapsed = 0;
        __block int ticks = 0;
        [NSTimer scheduledTimerWithTimeInterval:0.5 repeats:YES block:^(NSTimer *timer) {
            [compositor.surface setNeedsDisplay:YES];
            elapsed += 0.5;
            ticks += 1;
            if (ticks % 4 == 0) {
                NSLog(@"GL swaps: %ld (elapsed %.1fs)", gSwaps, elapsed);
            }
            if (elapsed >= 10) {
                [timer invalidate];
                printf("\n--- verdict ---\n");
                printf("GL swaps in ten seconds: %ld (the surface kept drawing: %s)\n", gSwaps,
                       gSwaps > 0 ? "yes" : "NO - the surface never drew");
                printf("Twenty swaps is two a second: this spike paints from a half-second timer\n"
                       "and a virtualised runner has no GPU worth the name, so the count is not a\n"
                       "performance claim. What matters is that it is not zero.\n");
                printf("Look at the window: is the title and the bar drawn OVER the moving colour?\n");
                printf("Click the picture: does the counter in the top right increase, with the\n"
                       "coordinates of the click? That is the OSD receiving a press through its\n"
                       "own transparent area, which is what the player's click-to-pause needs.\n");
                [NSApp terminate:nil];
            }
        }];
        [NSApp run];
    }
    return 0;
}
