dnl audit-macros.m4 - Architecture-aware audit rule macros
dnl
dnl Usage: m4 -DARCH={x86_64,aarch64} audit-macros.m4 rules-file.rules.m4
dnl
dnl Macro hierarchy:
dnl   SYSCALL         = SYSCALL_X86 + SYSCALL_ARM       (both arches, b32+b64)
dnl   SYSCALL_32      = SYSCALL_X86_32 + SYSCALL_ARM_32 (both arches, b32 only)
dnl   SYSCALL_64      = SYSCALL_X86_64 + SYSCALL_ARM_64 (both arches, b64 only)
dnl   SYSCALL_X86     = SYSCALL_X86_32 + SYSCALL_X86_64 (x86 only, b32+b64)
dnl   SYSCALL_ARM     = SYSCALL_ARM_32 + SYSCALL_ARM_64 (aarch64 only, b32+b64)
dnl
divert(-1)

dnl Leaf macros - emit a single rule line for a specific arch+width, or nothing
ifelse(ARCH,`aarch64',`
define(`SYSCALL_X86_32', `')
define(`SYSCALL_X86_64', `')
define(`SYSCALL_ARM_32', `-a always,exit -F arch=b32 -S $1 $2')
define(`SYSCALL_ARM_64', `-a always,exit -F arch=b64 -S $1 $2')
',`
define(`SYSCALL_X86_32', `-a always,exit -F arch=b32 -S $1 $2')
define(`SYSCALL_X86_64', `-a always,exit -F arch=b64 -S $1 $2')
define(`SYSCALL_ARM_32', `')
define(`SYSCALL_ARM_64', `')
')

dnl Composite macros - built from leaf macros
define(`SYSCALL_X86', `SYSCALL_X86_32($@)
SYSCALL_X86_64($@)')
define(`SYSCALL_ARM', `SYSCALL_ARM_32($@)
SYSCALL_ARM_64($@)')
define(`SYSCALL', `SYSCALL_X86($@)
SYSCALL_ARM($@)')
define(`SYSCALL_32', `SYSCALL_X86_32($@)
SYSCALL_ARM_32($@)')
define(`SYSCALL_64', `SYSCALL_X86_64($@)
SYSCALL_ARM_64($@)')

divert(0)dnl
