#include <czmq.h>
#include <stdio.h>

int main(void) {
    int major, minor, patch;
    zsys_version(&major, &minor, &patch);
    printf("CZMQ version: %d.%d.%d\n", major, minor, patch);
    return 0;
}
