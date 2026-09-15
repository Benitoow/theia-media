//go:build windows

package setup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows shortcuts: the entries the installer leaves in the Start Menu and on
// the Desktop, so that a person can find `theia-server` and `theia-player`
// without knowing where they were unpacked.
//
// A .lnk is not a text file holding a path. It is a serialised shell object, and
// the only thing that writes one a launcher (the Start Menu, Flow Launcher,
// Explorer) will follow is IShellLinkW. The near misses are worse than nothing:
// a .url opens the browser whatever it points at, and a .cmd flashes a console
// on the way. So the real format, written properly, or no entry at all.
//
// There is no CGO in this repository and never will be (spec-fondatrice §3), so
// shobjidl.h cannot be included: the interfaces are declared here by hand - their
// GUIDs, the position of each method in its vtable, and nothing else. Those
// positions are the entire risk of this file, which is why every index below is
// named and commented rather than written as a number.

// COM identifiers, spelled out as structures instead of parsed from strings at
// startup: CLSIDFromString fails at run time, and a shortcut that silently
// cannot be created is worse to diagnose than a wrong constant.
var (
	// CLSID_ShellLink {00021401-0000-0000-C000-000000000046}, the object that
	// knows the .lnk format.
	clsidShellLink = windows.GUID{
		Data1: 0x00021401, Data2: 0x0000, Data3: 0x0000,
		Data4: [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46},
	}
	// IID_IShellLinkW {000214F9-0000-0000-C000-000000000046}. The `W` matters:
	// asking for IShellLinkA gets an interface whose string parameters are ANSI,
	// and passing it UTF-16 is how a target with an accent in its path becomes a
	// shortcut to a file that does not exist.
	iidShellLinkW = windows.GUID{
		Data1: 0x000214F9, Data2: 0x0000, Data3: 0x0000,
		Data4: [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46},
	}
	// IID_IPersistFile {0000010B-0000-0000-C000-000000000046}, asked for
	// separately because writing to disk is not IShellLink's job: the link knows
	// what a shortcut says, IPersistFile knows how to store it.
	iidPersistFile = windows.GUID{
		Data1: 0x0000010B, Data2: 0x0000, Data3: 0x0000,
		Data4: [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46},
	}
)

// CoCreateInstance is not one of the functions golang.org/x/sys/windows wraps,
// so it is resolved here. NewLazySystemDLL loads it from System32 whatever the
// working directory is: an installer unpacked into a folder that also contains a
// file called ole32.dll must not load that one.
var ole32 = windows.NewLazySystemDLL("ole32.dll")

var procCoCreateInstance = ole32.NewProc("CoCreateInstance")

// Vtable positions, counted from the start of the interface.
//
// Every COM vtable begins with IUnknown - QueryInterface, AddRef, Release - so
// the first method an interface declares for itself is at index 3, not 0.
// Counting from the interface's own methods calls AddRef instead of SetPath,
// which returns S_OK and writes nothing: the shortcut is simply absent, with no
// error anywhere to explain it.
const (
	vtblQueryInterface = 0
	vtblRelease        = 2

	// IShellLinkW, in declaration order after IUnknown. The comment counts the
	// methods declared *before* the one named: a count that is one short calls
	// the getter next to the setter, whose extra out parameter is then a garbage
	// pointer - which is how SetIconLocation at 16 instead of 17 wrote through
	// GetIconLocation's `int *piIcon` and killed the process.
	vtblSetDescription      = 3 + 4  // GetPath, GetIDList, SetIDList, GetDescription precede it
	vtblSetWorkingDirectory = 3 + 6  // + GetWorkingDirectory
	vtblSetArguments        = 3 + 8  // + GetArguments
	vtblSetIconLocation     = 3 + 14 // + GetHotkey, SetHotkey, GetShowCmd, SetShowCmd, GetIconLocation
	vtblSetPath             = 3 + 17 // SetPath is declared last in IShellLinkW

	// IPersistFile: GetClassID is IPersist's own method, then IsDirty, Load and
	// Save, so Save is the fourth method after IUnknown.
	vtblSave = 3 + 3
)

// The two HRESULTs from CoInitializeEx that mean "somebody else already did
// this" rather than "this failed". An HRESULT is a signed 32-bit value, and
// RPC_E_CHANGED_MODE only fits in a uint32: it is compared below against the
// value truncated to its low 32 bits, which is the whole HRESULT.
const (
	hrFalse         = 0x00000001 // S_FALSE
	rpcEChangedMode = 0x80010106 // RPC_E_CHANGED_MODE
)

// Shortcut is one entry the installer creates for a person to click.
type Shortcut struct {
	Path        string // the .lnk file to write, absolute
	Target      string // the executable it starts
	Arguments   string
	WorkingDir  string
	Description string
	Icon        string // optional .ico or .exe; ignored when empty
}

// WriteShortcut creates the shortcut, replacing any file already there.
func WriteShortcut(s Shortcut) error {
	if s.Path == "" {
		return errors.New("setup: a shortcut needs a path to write the .lnk to")
	}
	if s.Target == "" {
		return errors.New("setup: a shortcut needs a target to start")
	}

	// A COM apartment belongs to the OS thread, not to the goroutine. The Go
	// runtime moves a goroutine between threads at any statement boundary, and a
	// CoCreateInstance made on a thread that never called CoInitializeEx answers
	// CO_E_NOTINITIALIZED - intermittently, under load, and never on the machine
	// where the code was written. Pinning the thread is what makes the call below
	// mean anything, and it is also why the matching CoUninitialize belongs to
	// this same function.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	ours, err := initializeCOM()
	if err != nil {
		return err
	}
	if ours {
		defer windows.CoUninitialize()
	}

	link, err := coCreateInstance()
	if err != nil {
		return err
	}
	defer releaseInterface(link)

	// In the order a person would read them off the shortcut's properties page.
	// Each of these copies its argument, so passing an empty string is harmless -
	// except for the icon, below.
	if err := setWideString(link, vtblSetPath, "IShellLinkW::SetPath", s.Target); err != nil {
		return err
	}
	if err := setWideString(link, vtblSetArguments, "IShellLinkW::SetArguments", s.Arguments); err != nil {
		return err
	}
	if err := setWideString(link, vtblSetWorkingDirectory, "IShellLinkW::SetWorkingDirectory", s.WorkingDir); err != nil {
		return err
	}
	if err := setWideString(link, vtblSetDescription, "IShellLinkW::SetDescription", s.Description); err != nil {
		return err
	}

	// An empty icon is not "the default icon": SetIconLocation with an empty path
	// leaves the shortcut pointing at an icon that is not there, and Explorer
	// draws its blank-page placeholder. Not calling it leaves the target's own
	// icon, which is what an entry without an icon is supposed to look like.
	if s.Icon != "" {
		if err := setIconLocation(link, s.Icon); err != nil {
			return err
		}
	}

	persist, err := queryInterface(link, &iidPersistFile)
	if err != nil {
		return err
	}
	defer releaseInterface(persist)

	// IPersistFile::Save answers STG_E_PATHNOTFOUND (0x80030003) instead of
	// creating a folder, and the folder a Start Menu entry belongs in is
	// Programs\Theia - which does not exist until this installer makes it.
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err != nil {
		return fmt.Errorf("setup: creating the folder for %s: %w", s.Path, err)
	}
	if err := save(persist, s.Path); err != nil {
		return err
	}

	// The link is released by the deferred call above, and liveness analysis is
	// free to consider a variable dead at its last syntactic use: this keeps the
	// object alive across every vtable call made before that release.
	runtime.KeepAlive(link)
	return nil
}

// initializeCOM puts this thread into a COM apartment. The returned bool says
// whether CoUninitialize has to be called, and it is only true when this call
// actually added a reference to an apartment.
func initializeCOM() (bool, error) {
	err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED)
	if err == nil {
		return true, nil
	}
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return false, fmt.Errorf("setup: CoInitializeEx: %w", err)
	}
	switch hr := uint32(errno); hr {
	case hrFalse:
		// S_FALSE: this thread is already in an apartment with the same model.
		// It is a success HRESULT and it took a reference, so it has to be given
		// back - skipping the uninitialize here is a leak, not a safety measure.
		return true, nil
	case rpcEChangedMode:
		// The thread is in an apartment with a different model, typically because
		// `theia-setup` was started from a shell that initialised COM itself.
		// Nothing was initialised by this call and there is no reference to
		// release; an unmatched CoUninitialize would tear down the apartment the
		// caller is still using. The existing apartment is usable as it is -
		// IShellLinkW is an in-process, free-threaded object.
		return false, nil
	default:
		return false, fmt.Errorf("setup: CoInitializeEx failed: HRESULT 0x%08X", hr)
	}
}

// coCreateInstance asks COM for a fresh, empty shortcut object. Only the
// in-process server is requested: the object lives in shell32, which is already
// loaded in any process that can show a desktop.
func coCreateInstance() (unsafe.Pointer, error) {
	var link unsafe.Pointer
	r1, _, _ := procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidShellLink)),
		0, // pUnkOuter: this object is not aggregated into another
		windows.CLSCTX_INPROC_SERVER,
		uintptr(unsafe.Pointer(&iidShellLinkW)),
		uintptr(unsafe.Pointer(&link)),
	)
	if err := hresult("CoCreateInstance(CLSID_ShellLink, IID_IShellLinkW)", r1); err != nil {
		return nil, err
	}
	if link == nil {
		return nil, errors.New("setup: CoCreateInstance returned no IShellLinkW")
	}
	return link, nil
}

// queryInterface asks an object for another of its interfaces. IPersistFile is
// not reachable from IShellLinkW any other way: they are two interfaces on one
// object, and the .lnk is only written through the second.
func queryInterface(iface unsafe.Pointer, iid *windows.GUID) (unsafe.Pointer, error) {
	var out unsafe.Pointer
	hr := comCall(iface, vtblQueryInterface,
		uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&out)))
	if err := hresult("IShellLinkW::QueryInterface(IID_IPersistFile)", hr); err != nil {
		return nil, err
	}
	if out == nil {
		return nil, errors.New("setup: QueryInterface returned no IPersistFile")
	}
	return out, nil
}

// setWideString calls the IShellLinkW setter at index that takes one wide string.
func setWideString(iface unsafe.Pointer, index uintptr, name, value string) error {
	wide, err := windows.UTF16PtrFromString(value)
	if err != nil {
		return fmt.Errorf("setup: %s: %w", name, err)
	}
	hr := comCall(iface, index, uintptr(unsafe.Pointer(wide)))
	// The conversion to uintptr above is opaque to the garbage collector, and
	// comCall is an ordinary Go function - it does not carry the
	// //go:uintptrkeepalive that syscall.SyscallN does. Without this the UTF-16
	// buffer, which UTF16PtrFromString allocates on the heap, can be collected
	// while COM is still reading it, and the .lnk ends up holding a path that was
	// never passed to it.
	runtime.KeepAlive(wide)
	return hresult(name, hr)
}

// setIconLocation takes a path and an index into that file's icons; index 0 is
// the first icon, which is the only one Theia has to offer.
func setIconLocation(iface unsafe.Pointer, icon string) error {
	const name = "IShellLinkW::SetIconLocation"
	wide, err := windows.UTF16PtrFromString(icon)
	if err != nil {
		return fmt.Errorf("setup: %s: %w", name, err)
	}
	hr := comCall(iface, vtblSetIconLocation, uintptr(unsafe.Pointer(wide)), 0)
	runtime.KeepAlive(wide)
	return hresult(name, hr)
}

// save stores the link on disk.
//
// remember must be TRUE (1). With FALSE the file is still written, but the object
// does not adopt it as its current file and goes on considering itself dirty -
// a state the shell's own examples never create, and whose symptoms appear
// somewhere other than at the call that got it wrong.
func save(persist unsafe.Pointer, path string) error {
	const name = "IPersistFile::Save"
	wide, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return fmt.Errorf("setup: %s: %w", name, err)
	}
	hr := comCall(persist, vtblSave, uintptr(unsafe.Pointer(wide)), 1)
	runtime.KeepAlive(wide)
	return hresult(name, hr)
}

// comCall invokes the method at index in the vtable belonging to iface. args are
// passed to it as machine words, with iface itself as the implicit this pointer.
func comCall(iface unsafe.Pointer, index uintptr, args ...uintptr) uintptr {
	// The interface pointer points at the vtable pointer, and the methods are
	// consecutive words from there. There is no header, no length and no bounds
	// check: index is the whole contract, and one that is too large is a call
	// through whatever happens to follow the table.
	//
	// unsafe.Add rather than holding the vtable address in a uintptr and adding to
	// it: go vet's unsafeptr check refuses that round trip, and it is the one the
	// unsafe.Pointer rules exist to forbid - an integer is not a pointer, and
	// nothing keeps the object alive across the arithmetic.
	vtbl := *(*unsafe.Pointer)(iface)
	fn := *(*uintptr)(unsafe.Add(vtbl, index*unsafe.Sizeof(uintptr(0))))
	r1, _, _ := syscall.SyscallN(fn, append([]uintptr{uintptr(iface)}, args...)...)
	return r1
}

// releaseInterface drops the reference that CoCreateInstance or QueryInterface
// handed out: both give the caller an owned reference, and an interface pointer
// that is never released keeps shell32's object alive for the life of the
// process. Not called `release`: internal/release is already imported in this
// package, and a package-level declaration may not share a file's import name.
func releaseInterface(iface unsafe.Pointer) {
	if iface != nil {
		comCall(iface, vtblRelease)
	}
}

// hresult turns a call's return value into an error naming the call, so that a
// failure says which one it was instead of leaving the next person to guess. A
// negative value is a failure; only the low 32 bits are the HRESULT, the rest of
// the register is not part of the answer.
func hresult(name string, value uintptr) error {
	hr := int32(uint32(value))
	if hr >= 0 {
		return nil
	}
	return fmt.Errorf("setup: %s failed: HRESULT 0x%08X", name, uint32(hr))
}

// StartMenuDir is where a per-user Start Menu entry belongs.
//
// %APPDATA% rather than SHGetKnownFolderPath(FOLDERID_Programs): it is the same
// folder, it is the variable the Startup entry is already built from
// (startupEntryPath), and reading it is what lets a test redirect the Start Menu
// into a temporary directory instead of writing into the real one.
func StartMenuDir() (string, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return "", errors.New("setup: APPDATA is not set, so the Start Menu folder cannot be found")
	}
	return filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs"), nil
}

// DesktopDir is the user's Desktop, asked of the shell rather than assembled
// from %USERPROFILE%.
//
// %USERPROFILE%\Desktop is the wrong answer on most Windows 11 machines:
// OneDrive takes the Desktop over and moves it to
// %USERPROFILE%\OneDrive\Desktop, and the real path is also localised. A
// shortcut written to the guessed path is not lost - it is in a folder the
// person never opens, which for this feature is the same thing.
func DesktopDir() (string, error) {
	path, err := windows.KnownFolderPath(windows.FOLDERID_Desktop, windows.KF_FLAG_DEFAULT)
	if err != nil {
		return "", fmt.Errorf("setup: SHGetKnownFolderPath(FOLDERID_Desktop): %w", err)
	}
	return path, nil
}
