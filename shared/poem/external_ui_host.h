#pragma once
#include <stdint.h>

// Stable C ABI exported by an application-specific POEM c-shared module.
// An external native owner may host this runtime without using POEM's window
// or presenters. Read/write retain the four-byte framed POEM protocol.
#ifdef __cplusplus
extern "C" {
#endif
uint32_t PoemWindowsABIVersion(void);
int32_t PoemWindowsMetadata(void* buffer,int32_t capacity);
int32_t PoemWindowsStart(int32_t logical_width,int32_t logical_height);
int32_t PoemHostRead(void* buffer,int32_t capacity);
int32_t PoemHostWrite(const void* buffer,int32_t length);
void PoemWindowsStop(void);
#ifdef __cplusplus
}
#endif

