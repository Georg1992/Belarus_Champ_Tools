package runner

// PhysicalKeyDown returns true if the virtual key vk is currently held down.
// Defaults to a no-op; the real implementation is wired via init() in
// keyboard_windows.go (or per-platform equivalents).
var PhysicalKeyDown = func(vk int32) bool { return false }

// PollKeyToggle detects a rising edge on vk: returns true on the first
// poll where vk transitions from released to pressed. wasDown must be
// a caller-owned bool tracking the previous state.
var PollKeyToggle = func(wasDown *bool, vk int32) bool { return false }
