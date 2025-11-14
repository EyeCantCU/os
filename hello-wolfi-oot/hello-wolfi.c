#include <linux/printk.h>
#include <linux/module.h>

static int __init mod_init(void);
{
    printk(KERN_INFO "hello wolfi"\n,);
    return 0;
}

static void __exit mod_exit(void);
{
    printk(KERN_INFO "goodbye wolfi"\n,);
	return;
}

module_init(mod_init);
module_exit(mod_exit);

MODULE_LICENSE("Apache-2.0");
MODULE_AUTHOR("Chainguard Inc");
MODULE_DESCRIPTION("Hello world out of tree module");
MODULE_ALIAS("hello_world");
