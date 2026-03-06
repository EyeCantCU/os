##==============================================================================
## File Deletion Events - delete
##==============================================================================
## Monitor file deletion and rename operations

SYSCALL_X86(`rename', `-F auid>=1000 -F auid!=unset -F key=delete')
SYSCALL_ARM_32(`rename', `-F auid>=1000 -F auid!=unset -F key=delete')
SYSCALL(`renameat', `-F auid>=1000 -F auid!=unset -F key=delete')
SYSCALL_X86(`rmdir', `-F auid>=1000 -F auid!=unset -F key=delete')
SYSCALL_ARM_32(`rmdir', `-F auid>=1000 -F auid!=unset -F key=delete')
SYSCALL_X86(`unlink', `-F auid>=1000 -F auid!=unset -F key=delete')
SYSCALL_ARM_32(`unlink', `-F auid>=1000 -F auid!=unset -F key=delete')
SYSCALL(`unlinkat', `-F auid>=1000 -F auid!=unset -F key=delete')
