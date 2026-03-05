##==============================================================================
## Unsuccessful File Modification - access
##==============================================================================
## Monitor failed attempts to access/modify files (permission denied)

SYSCALL_X86(`creat', `-F exit=-EACCES -F auid>=1000 -F auid!=unset -F key=access')
SYSCALL_ARM_32(`creat', `-F exit=-EACCES -F auid>=1000 -F auid!=unset -F key=access')
SYSCALL_X86(`creat', `-F exit=-EPERM -F auid>=1000 -F auid!=unset -F key=access')
SYSCALL_ARM_32(`creat', `-F exit=-EPERM -F auid>=1000 -F auid!=unset -F key=access')
SYSCALL(`ftruncate', `-F exit=-EACCES -F auid>=1000 -F auid!=unset -F key=access')
SYSCALL(`ftruncate', `-F exit=-EPERM -F auid>=1000 -F auid!=unset -F key=access')
SYSCALL_X86(`open', `-F exit=-EACCES -F auid>=1000 -F auid!=unset -F key=access')
SYSCALL_ARM_32(`open', `-F exit=-EACCES -F auid>=1000 -F auid!=unset -F key=access')
SYSCALL_X86(`open', `-F exit=-EPERM -F auid>=1000 -F auid!=unset -F key=access')
SYSCALL_ARM_32(`open', `-F exit=-EPERM -F auid>=1000 -F auid!=unset -F key=access')
SYSCALL(`open_by_handle_at,truncate,ftruncate', `-F exit=-EACCES -F auid>=1000 -F auid!=unset -F key=access')
SYSCALL(`open_by_handle_at,truncate,ftruncate', `-F exit=-EPERM -F auid>=1000 -F auid!=unset -F key=access')
SYSCALL(`openat', `-F exit=-EACCES -F auid>=1000 -F auid!=unset -F key=access')
SYSCALL(`openat', `-F exit=-EPERM -F auid>=1000 -F auid!=unset -F key=access')
SYSCALL(`truncate', `-F exit=-EACCES -F auid>=1000 -F auid!=unset -F key=access')
SYSCALL(`truncate', `-F exit=-EPERM -F auid>=1000 -F auid!=unset -F key=access')
