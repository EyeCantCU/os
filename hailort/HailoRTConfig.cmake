get_filename_component(_pkgdir "${CMAKE_CURRENT_LIST_DIR}/../../.." ABSOLUTE)
set(HailoRT_INCLUDE_DIR "${_pkgdir}/include")

# Try lib first, then lib64 (works regardless of where this file lives)
if(EXISTS "${_pkgdir}/lib/libhailort.so")
set(HailoRT_LIBRARY "${_pkgdir}/lib/libhailort.so")
elseif(EXISTS "${_pkgdir}/lib64/libhailort.so")
set(HailoRT_LIBRARY "${_pkgdir}/lib64/libhailort.so")
endif()

if(NOT EXISTS "${HailoRT_INCLUDE_DIR}/hailo/hailort.hpp" OR NOT EXISTS "${HailoRT_LIBRARY}")
message(FATAL_ERROR "HailoRT headers or library not found under ${_pkgdir}")
endif()

if(NOT TARGET HailoRT::libhailort)
add_library(HailoRT::libhailort SHARED IMPORTED)
set_target_properties(HailoRT::libhailort PROPERTIES
IMPORTED_LOCATION "${HailoRT_LIBRARY}"
INTERFACE_INCLUDE_DIRECTORIES "${HailoRT_INCLUDE_DIR}")
endif()

set(HailoRT_FOUND TRUE)