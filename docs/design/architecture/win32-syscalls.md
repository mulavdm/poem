# Win32 Syscall Stabilization (The "Success" Trap)

A critical hurdle in building native Win32 wrappers in Go is handling `syscall.Proc.Call` return values. In Go, these calls always return a non-nil `error`, which defaults to `"The operation completed successfully"` (Errno 0).

## The Bug

Standard Go error checking (`if err != nil`) causes panics on success if not handled carefully.

## The Solution: Deterministic Error Wrappers

`internal/win32` respects the Win32 API contract instead:

```go
func CreateWindow(...) (uintptr, error) {
    ret, _, err := procCreateWindow.Call(...)
    if ret == 0 { // In Win32, 0 usually indicates failure
        return 0, err
    }
    return ret, nil // Explicitly return nil on success
}
```

## See also
- [Architecture Overview](TDD.md)
- [Thread Locking](../architecture/thread-locking.md)
