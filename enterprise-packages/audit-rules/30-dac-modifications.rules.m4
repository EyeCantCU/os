##==============================================================================
## DAC Modification (Permission/Ownership) - perm_mod
##==============================================================================
## Monitor changes to file permissions and ownership

SYSCALL_X86(`chmod', `-F auid>=1000 -F auid!=unset -F key=perm_mod')
SYSCALL_ARM_32(`chmod', `-F auid>=1000 -F auid!=unset -F key=perm_mod')
SYSCALL(`fchmod', `-F auid>=1000 -F auid!=unset -F key=perm_mod')
SYSCALL(`fchmodat', `-F auid>=1000 -F auid!=unset -F key=perm_mod')

SYSCALL_X86(`chown', `-F auid>=1000 -F auid!=unset -F key=perm_mod')
SYSCALL_ARM_32(`chown', `-F auid>=1000 -F auid!=unset -F key=perm_mod')
SYSCALL(`fchown', `-F auid>=1000 -F auid!=unset -F key=perm_mod')
SYSCALL(`fchownat', `-F auid>=1000 -F auid!=unset -F key=perm_mod')
SYSCALL_X86(`lchown', `-F auid>=1000 -F auid!=unset -F key=perm_mod')
SYSCALL_ARM_32(`lchown', `-F auid>=1000 -F auid!=unset -F key=perm_mod')

SYSCALL(`fremovexattr', `-F auid>=1000 -F auid!=unset -F key=perm_mod')
SYSCALL(`fsetxattr', `-F auid>=1000 -F auid!=unset -F key=perm_mod')
SYSCALL(`lremovexattr', `-F auid>=1000 -F auid!=unset -F key=perm_mod')
SYSCALL(`lsetxattr', `-F auid>=1000 -F auid!=unset -F key=perm_mod')
SYSCALL(`removexattr', `-F auid>=1000 -F auid!=unset -F key=perm_mod')
SYSCALL(`setxattr', `-F auid>=1000 -F auid!=unset -F key=perm_mod')

SYSCALL_32(`umount', `-F auid>=1000 -F auid!=unset -F key=perm_mod')
SYSCALL(`umount2', `-F auid>=1000 -F auid!=unset -F key=perm_mod')
