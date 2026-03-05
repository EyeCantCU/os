##==============================================================================
## Kernel Module Loading - modules
##==============================================================================
## Monitor loading and unloading of kernel modules

SYSCALL(`delete_module', `-F key=modules')
SYSCALL(`finit_module', `-F key=modules')
SYSCALL(`init_module', `-F key=modules')
