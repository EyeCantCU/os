#include <stdio.h>
#include <stdlib.h>
#include <ta-lib/ta_libc.h>

int main() {
    TA_RetCode retCode;
    int outBegIdx;
    int outNBElement;

    // Initialize TA-Lib
    retCode = TA_Initialize();
    if (retCode != TA_SUCCESS) {
        fprintf(stderr, "TA_Initialize failed: %d\n", retCode);
        return 1;
    }

    // Test data: simple price series
    double closePrice[] = {10.0, 11.0, 12.0, 13.0, 14.0, 15.0, 16.0, 17.0, 18.0, 19.0};
    double sma[10];
    int period = 5;

    // Calculate Simple Moving Average
    retCode = TA_SMA(0, 9, closePrice, period, &outBegIdx, &outNBElement, sma);
    if (retCode != TA_SUCCESS) {
        fprintf(stderr, "TA_SMA failed: %d\n", retCode);
        TA_Shutdown();
        return 1;
    }

    printf("SMA calculation successful\n");
    printf("Output Begin Index: %d\n", outBegIdx);
    printf("Number of Elements: %d\n", outNBElement);
    printf("SMA values:\n");
    for (int i = 0; i < outNBElement; i++) {
        printf("  SMA[%d] = %.2f\n", i, sma[i]);
    }

    // Verify the first SMA value (average of first 5 prices: 10-14)
    double expected_first_sma = (10.0 + 11.0 + 12.0 + 13.0 + 14.0) / 5.0;
    if (sma[0] < expected_first_sma - 0.01 || sma[0] > expected_first_sma + 0.01) {
        fprintf(stderr, "SMA calculation incorrect: expected %.2f, got %.2f\n",
                expected_first_sma, sma[0]);
        TA_Shutdown();
        return 1;
    }

    printf("SMA value verification passed\n");

    // Test RSI calculation
    double rsi[10];
    retCode = TA_RSI(0, 9, closePrice, 14, &outBegIdx, &outNBElement, rsi);
    if (retCode != TA_SUCCESS) {
        fprintf(stderr, "TA_RSI failed: %d\n", retCode);
        TA_Shutdown();
        return 1;
    }

    printf("RSI calculation successful\n");
    printf("RSI values: %d elements\n", outNBElement);

    // Cleanup
    retCode = TA_Shutdown();
    if (retCode != TA_SUCCESS) {
        fprintf(stderr, "TA_Shutdown failed: %d\n", retCode);
        return 1;
    }

    printf("All tests passed!\n");
    return 0;
}
