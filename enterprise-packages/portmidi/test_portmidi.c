#include <stdio.h>
#include <portmidi.h>

int main() {
    PmError err;

    err = Pm_Initialize();
    if (err != pmNoError) {
        printf("Pm_Initialize failed: %s\n", Pm_GetErrorText(err));
        return 1;
    }
    printf("PortMidi initialized successfully\n");

    int count = Pm_CountDevices();
    printf("Found %d MIDI device(s)\n", count);

    err = Pm_Terminate();
    if (err != pmNoError) {
        printf("Pm_Terminate failed: %s\n", Pm_GetErrorText(err));
        return 1;
    }
    printf("PortMidi terminated successfully\n");

    return 0;
}
