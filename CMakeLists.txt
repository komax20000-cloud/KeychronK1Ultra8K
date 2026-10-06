cmake_minimum_required(VERSION 3.13.1)

target_sources_ifdef(CONFIG_ZMK_KEY_REPEAT app PRIVATE src/key_auto_repeat.c)

if(CONFIG_ZMK_KEY_REPEAT AND CONFIG_LED_STRIP)
  target_sources(app PRIVATE src/key_repeat_led.c)
  zephyr_ld_options(-Wl,--wrap=zmk_rgb_matrix_update_pwm_buffers)
endif()
