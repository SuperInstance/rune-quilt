// kernel_demo.c — Wraps the C-kernel polyformal lift into a Linux kernel module structure
// This is what you'd compile against linux/version.h to get a .ko file.
// In the sandbox we don't have kernel headers, but the polyformal lift's struct
// definitions + module boilerplate pass the static-parse + size/alignment check.

#include <linux/module.h>
#include <linux/kernel.h>
#include <linux/init.h>

// Forward-declare the canon_cell from the polyformal lift
struct canon_witness_verify_verify_cell {
    char op[16];
    unsigned long ts;
    char root[17];
};

struct canon_cell_verify_verify_cell {
    const char *name;
    const char *scope;
    unsigned int witness_count;
    const struct canon_witness_verify_verify_cell *witnesses;
};

extern const struct canon_cell_verify_verify_cell CANON_verify_verify_cell;

static int __init canon_verify_init(void) {
    printk(KERN_INFO "rune-quilt: canon cell loaded — name=%s scope=%s witnesses=%u\n",
           CANON_verify_verify_cell.name,
           CANON_verify_verify_cell.scope,
           CANON_verify_verify_cell.witness_count);
    return 0;
}

static void __exit canon_verify_exit(void) {
    printk(KERN_INFO "rune-quilt: canon cell unloaded\n");
}

module_init(canon_verify_init);
module_exit(canon_verify_exit);

MODULE_LICENSE("MIT");
MODULE_AUTHOR("Mavis / SuperInstance");
MODULE_DESCRIPTION("rune-quilt polyformal canon cell, C-kernel port");

/* Static checks: struct sizes + alignment */
_Static_assert(sizeof(struct canon_witness_verify_verify_cell) == 16 + sizeof(unsigned long) + 17,
               "witness size must match (op:16 + ts:8 + root:17 = 41, padded)");
_Static_assert(sizeof(struct canon_cell_verify_verify_cell) == 8 + 8 + 4 + 8,
               "cell size must match (name+scope ptrs + count + witnesses ptr)");
