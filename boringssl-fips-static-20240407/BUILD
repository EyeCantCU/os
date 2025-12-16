cc_library(
    name = "crypto",
    srcs = [
        "libcrypto.a",
    ],
    hdrs = glob(["include/openssl/*.h"]),
    defines = ["BORINGSSL_FIPS"],
    includes = ["include"],
    visibility = ["//visibility:public"],
)

cc_library(
    name = "ssl",
    srcs = [
        "libssl.a",
    ],
    hdrs = glob(["include/openssl/*.h"]),
    includes = ["include"],
    visibility = ["//visibility:public"],
    deps = [":crypto"],
)
