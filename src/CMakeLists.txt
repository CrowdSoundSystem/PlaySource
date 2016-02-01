cmake_minimum_required(VERSION 2.8.0)
project(playsource)

include_directories(.)

file(GLOB_RECURSE playsource_srcs
    "*.cpp"
)

add_library(playsource STATIC ${playsource_srcs})

set_property(TARGET playsource PROPERTY CXX_STANDARD 11)
set_target_properties(playsource PROPERTIES PREFIX "")
install(TARGETS playsource DESTINATION bin)
