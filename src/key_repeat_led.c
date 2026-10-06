/*
 * While key auto-repeat is enabled, force the F1-F12 row to pulse as a wave and
 * keep the Up key lit, regardless of the active RGB effect.
 *
 * The overlay is applied just before the LED buffer is flushed by wrapping
 * zmk_rgb_matrix_update_pwm_buffers() (see --wrap in CMakeLists.txt).
 */

#include <zephyr/kernel.h>
#include <zephyr/sys/util.h>
#include "rgb_matrix.h"

#if IS_ENABLED(CONFIG_ZMK_KEY_REPEAT)

/* LED indices from g_led_config (rgb_matrix_config.h) */
#define F1_LED_INDEX 1
#define F12_LED_INDEX 12
#define UP_LED_INDEX 73

#define WAVE_STEP_MS 4   /* one phase step; full cycle = 256 * 4 ms */
#define WAVE_LED_SHIFT 20 /* phase offset between neighbouring keys */
#define WAVE_MIN_VAL 24
#define UP_MIN_VAL 128

extern bool key_auto_repeat_is_enabled(void);
extern void __real_zmk_rgb_matrix_update_pwm_buffers(void);

static void set_hsv(int led, uint8_t val) {
    HSV hsv = {rgb_matrix_config.hsv.h, rgb_matrix_config.hsv.s, val};
    RGB rgb = rgb_matrix_hsv_to_rgb(hsv);

    zmk_rgb_matrix_set_color(led, rgb.r, rgb.g, rgb.b);
}

void __wrap_zmk_rgb_matrix_update_pwm_buffers(void) {
    if (key_auto_repeat_is_enabled() && rgb_matrix_config.enable &&
        !zmk_rgb_matrix_get_suspend_state()) {
        uint8_t base = rgb_matrix_config.hsv.v;
        uint32_t t = (uint32_t)(k_uptime_get_32() / WAVE_STEP_MS);

        for (int i = F1_LED_INDEX; i <= F12_LED_INDEX; i++) {
            uint8_t phase = (uint8_t)(t - (i - F1_LED_INDEX) * WAVE_LED_SHIFT);
            uint8_t tri = phase < 128 ? phase * 2 : (255 - phase) * 2;
            uint8_t level = WAVE_MIN_VAL + ((255 - WAVE_MIN_VAL) * tri) / 255;

            set_hsv(i, (base * level) / 255);
        }

        set_hsv(UP_LED_INDEX, MAX(base, UP_MIN_VAL));
    }

    __real_zmk_rgb_matrix_update_pwm_buffers();
}

#endif
