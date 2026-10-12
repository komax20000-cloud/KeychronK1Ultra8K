cmake_minimum_required(VERSION 3.13.1)

target_sources_ifdef(CONFIG_ZMK_KEY_REPEAT app PRIVATE src/key_auto_repeat.c)
