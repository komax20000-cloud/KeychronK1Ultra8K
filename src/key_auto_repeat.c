/*
 * Device-side auto-repeat for held keyboard keys, controlled by the
 * repeat_toggle / repeat_delay / repeat_rate / repeat_pause behaviors.
 */

#include <zephyr/device.h>
#include <zephyr/kernel.h>
#include <zephyr/logging/log.h>
#include <drivers/behavior.h>
#include <zmk/behavior.h>
#include <zmk/event_manager.h>
#include <zmk/events/keycode_state_changed.h>

LOG_MODULE_DECLARE(zmk, CONFIG_ZMK_LOG_LEVEL);

#define DT_REPEAT_ANY                                                                              \
    (DT_HAS_COMPAT_STATUS_OKAY(zmk_behavior_key_repeat_toggle) ||                                  \
     DT_HAS_COMPAT_STATUS_OKAY(zmk_behavior_key_repeat_delay) ||                                   \
     DT_HAS_COMPAT_STATUS_OKAY(zmk_behavior_key_repeat_rate) ||                                    \
     DT_HAS_COMPAT_STATUS_OKAY(zmk_behavior_key_repeat_pause) ||                                   \
     DT_HAS_COMPAT_STATUS_OKAY(zmk_behavior_key_repeat_multi))

#if DT_REPEAT_ANY && IS_ENABLED(CONFIG_ZMK_KEY_REPEAT)

#define DEFAULT_DELAY_MS 20
#define MAX_DELAY_MS 1000
#define DEFAULT_RATE_MS 30
#define MIN_RATE_MS 10
#define MAX_RATE_MS 100
#define RATE_STEP_MS 5
#define PAUSE_MS 5000
#define MAX_KEYS 6

#define HID_KEY_CAPS_LOCK 0x39
#define HID_KEY_SCROLL_LOCK 0x47
#define HID_KEY_NUM_LOCK 0x53
#define HID_KEY_LOCKING_CAPS 0x82
#define HID_KEY_LOCKING_NUM 0x83
#define HID_KEY_LOCKING_SCROLL 0x84

static bool enabled;
static bool multi; /* repeat every held key, not just the newest; only effective while enabled */
static bool paused;
static bool emitting;
static uint16_t delay_ms = DEFAULT_DELAY_MS;
static uint16_t rate_ms = DEFAULT_RATE_MS;

struct repeat_slot {
    bool used;
    uint32_t order; /* press sequence number, higher = newer */
    int64_t next_ms;
    struct zmk_keycode_state_changed ev;
};

static struct repeat_slot slots[MAX_KEYS];
static uint32_t press_seq;

static void repeat_work_handler(struct k_work *work);
static void resume_work_handler(struct k_work *work);
static K_WORK_DELAYABLE_DEFINE(repeat_work, repeat_work_handler);
static K_WORK_DELAYABLE_DEFINE(resume_work, resume_work_handler);

static bool multi_active(void) { return enabled && multi; }

static bool is_repeatable(const struct zmk_keycode_state_changed *ev) {
    if (ev->usage_page != HID_USAGE_KEY || ev->explicit_modifiers != 0 ||
        is_mod(ev->usage_page, ev->keycode)) {
        return false;
    }

    switch (ev->keycode) {
    case HID_KEY_CAPS_LOCK:
    case HID_KEY_SCROLL_LOCK:
    case HID_KEY_NUM_LOCK:
    case HID_KEY_LOCKING_CAPS:
    case HID_KEY_LOCKING_NUM:
    case HID_KEY_LOCKING_SCROLL:
        return false;
    default:
        return true;
    }
}

static void emit(const struct zmk_keycode_state_changed *src, bool pressed) {
    struct zmk_keycode_state_changed ev = *src;

    ev.state = pressed;
    ev.timestamp = k_uptime_get();
    emitting = true;
    raise_zmk_keycode_state_changed(ev);
    emitting = false;
}

static struct repeat_slot *find_slot(const struct zmk_keycode_state_changed *ev) {
    for (int i = 0; i < MAX_KEYS; i++) {
        if (slots[i].used && slots[i].ev.usage_page == ev->usage_page &&
            slots[i].ev.keycode == ev->keycode) {
            return &slots[i];
        }
    }
    return NULL;
}

/* Re-arms the single work item for the earliest pending slot, or cancels it if none */
static void schedule_next(void) {
    int64_t soonest = INT64_MAX;

    for (int i = 0; i < MAX_KEYS; i++) {
        if (slots[i].used && slots[i].next_ms < soonest) {
            soonest = slots[i].next_ms;
        }
    }

    if (soonest == INT64_MAX) {
        k_work_cancel_delayable(&repeat_work);
    } else {
        k_work_reschedule(&repeat_work, K_MSEC(MAX(soonest - k_uptime_get(), 0)));
    }
}

static void clear_all(void) {
    for (int i = 0; i < MAX_KEYS; i++) {
        slots[i].used = false;
    }
    k_work_cancel_delayable(&repeat_work);
}

/* Leaves only the most recently pressed key repeating (single-key mode) */
static void keep_newest_only(void) {
    struct repeat_slot *newest = NULL;

    for (int i = 0; i < MAX_KEYS; i++) {
        if (slots[i].used && (newest == NULL || slots[i].order > newest->order)) {
            newest = &slots[i];
        }
    }
    for (int i = 0; i < MAX_KEYS; i++) {
        if (&slots[i] != newest) {
            slots[i].used = false;
        }
    }
    schedule_next();
}

static void add_slot(const struct zmk_keycode_state_changed *ev) {
    struct repeat_slot *slot = find_slot(ev);

    for (int i = 0; slot == NULL && i < MAX_KEYS; i++) {
        if (!slots[i].used) {
            slot = &slots[i];
        }
    }
    /* Table full: replace the oldest key */
    if (slot == NULL) {
        slot = &slots[0];
        for (int i = 1; i < MAX_KEYS; i++) {
            if (slots[i].order < slot->order) {
                slot = &slots[i];
            }
        }
    }

    slot->used = true;
    slot->order = ++press_seq;
    slot->ev = *ev;
    slot->next_ms = k_uptime_get() + delay_ms;
}

static void repeat_work_handler(struct k_work *work) {
    if (!enabled || paused) {
        clear_all();
        return;
    }

    int64_t now = k_uptime_get();

    for (int i = 0; i < MAX_KEYS; i++) {
        if (slots[i].used && slots[i].next_ms <= now) {
            struct zmk_keycode_state_changed ev = slots[i].ev;

            slots[i].next_ms = now + rate_ms;
            emit(&ev, false);
            emit(&ev, true);
        }
    }
    schedule_next();
}

static void resume_work_handler(struct k_work *work) { paused = false; }

static int key_auto_repeat_listener(const zmk_event_t *eh) {
    struct zmk_keycode_state_changed *ev = as_zmk_keycode_state_changed(eh);

    if (ev == NULL || emitting) {
        return ZMK_EV_EVENT_BUBBLE;
    }

    if (ev->state) {
        /* Single-key mode: any new press ends the previous key's repeat.
         * Multi-key mode: only repeatable presses are tracked; others leave the rest alone. */
        if (!multi_active()) {
            clear_all();
        }
        if (enabled && !paused && is_repeatable(ev)) {
            add_slot(ev);
            schedule_next();
        }
    } else {
        struct repeat_slot *slot = find_slot(ev);

        if (slot != NULL) {
            slot->used = false;
            schedule_next();
        }
    }

    return ZMK_EV_EVENT_BUBBLE;
}

ZMK_LISTENER(key_auto_repeat, key_auto_repeat_listener);
ZMK_SUBSCRIPTION(key_auto_repeat, zmk_keycode_state_changed);

bool key_auto_repeat_is_enabled(void) { return enabled; }

static int repeat_init(const struct device *dev) { return 0; }

static int repeat_released(struct zmk_behavior_binding *binding,
                           struct zmk_behavior_binding_event event) {
    return ZMK_BEHAVIOR_OPAQUE;
}

#define REPEAT_API(name)                                                                           \
    static const struct behavior_driver_api name##_api = {                                         \
        .binding_pressed = name##_pressed,                                                         \
        .binding_released = repeat_released,                                                       \
    }

#define REPEAT_INST(n)                                                                             \
    BEHAVIOR_DT_INST_DEFINE(n, repeat_init, NULL, NULL, NULL, POST_KERNEL,                         \
                            CONFIG_KERNEL_INIT_PRIORITY_DEFAULT, &DT_DRV_API_NAME);

/* repeat_toggle <0|1> */
#define DT_DRV_COMPAT zmk_behavior_key_repeat_toggle
#if DT_HAS_COMPAT_STATUS_OKAY(DT_DRV_COMPAT)
static int toggle_pressed(struct zmk_behavior_binding *binding,
                          struct zmk_behavior_binding_event event) {
    enabled = binding->param1 != 0;
    /* Multi-key repeat follows the main switch: on with repeat, off with repeat */
    multi = enabled;
    if (!enabled) {
        clear_all();
    }
    return ZMK_BEHAVIOR_OPAQUE;
}
REPEAT_API(toggle);
#undef DT_DRV_API_NAME
#define DT_DRV_API_NAME toggle_api
DT_INST_FOREACH_STATUS_OKAY(REPEAT_INST)
#endif
#undef DT_DRV_COMPAT

/* repeat_multi <0 = off | 1 = on | 2 = toggle>: repeat all held keys. Ignored while repeat is off. */
#define DT_DRV_COMPAT zmk_behavior_key_repeat_multi
#if DT_HAS_COMPAT_STATUS_OKAY(DT_DRV_COMPAT)
static int multi_pressed(struct zmk_behavior_binding *binding,
                         struct zmk_behavior_binding_event event) {
    if (!enabled) {
        return ZMK_BEHAVIOR_OPAQUE;
    }
    multi = binding->param1 == 2 ? !multi : binding->param1 != 0;
    if (!multi) {
        keep_newest_only();
    }
    return ZMK_BEHAVIOR_OPAQUE;
}
REPEAT_API(multi);
#undef DT_DRV_API_NAME
#define DT_DRV_API_NAME multi_api
DT_INST_FOREACH_STATUS_OKAY(REPEAT_INST)
#endif
#undef DT_DRV_COMPAT

/* repeat_delay <ms> */
#define DT_DRV_COMPAT zmk_behavior_key_repeat_delay
#if DT_HAS_COMPAT_STATUS_OKAY(DT_DRV_COMPAT)
static int delay_pressed(struct zmk_behavior_binding *binding,
                         struct zmk_behavior_binding_event event) {
    delay_ms = MIN(binding->param1, MAX_DELAY_MS);
    return ZMK_BEHAVIOR_OPAQUE;
}
REPEAT_API(delay);
#undef DT_DRV_API_NAME
#define DT_DRV_API_NAME delay_api
DT_INST_FOREACH_STATUS_OKAY(REPEAT_INST)
#endif
#undef DT_DRV_COMPAT

/* repeat_rate <0 = faster | 1 = slower> */
#define DT_DRV_COMPAT zmk_behavior_key_repeat_rate
#if DT_HAS_COMPAT_STATUS_OKAY(DT_DRV_COMPAT)
static int rate_pressed(struct zmk_behavior_binding *binding,
                        struct zmk_behavior_binding_event event) {
    if (binding->param1 == 0) {
        rate_ms = rate_ms > MIN_RATE_MS + RATE_STEP_MS ? rate_ms - RATE_STEP_MS : MIN_RATE_MS;
    } else {
        rate_ms = MIN(rate_ms + RATE_STEP_MS, MAX_RATE_MS);
    }
    return ZMK_BEHAVIOR_OPAQUE;
}
REPEAT_API(rate);
#undef DT_DRV_API_NAME
#define DT_DRV_API_NAME rate_api
DT_INST_FOREACH_STATUS_OKAY(REPEAT_INST)
#endif
#undef DT_DRV_COMPAT

/* repeat_pause: suspends repeat for 5 s; pressing again restarts the countdown */
#define DT_DRV_COMPAT zmk_behavior_key_repeat_pause
#if DT_HAS_COMPAT_STATUS_OKAY(DT_DRV_COMPAT)
static int pause_pressed(struct zmk_behavior_binding *binding,
                         struct zmk_behavior_binding_event event) {
    paused = true;
    clear_all();
    k_work_reschedule(&resume_work, K_MSEC(PAUSE_MS));
    return ZMK_BEHAVIOR_OPAQUE;
}
REPEAT_API(pause);
#undef DT_DRV_API_NAME
#define DT_DRV_API_NAME pause_api
DT_INST_FOREACH_STATUS_OKAY(REPEAT_INST)
#endif
#undef DT_DRV_COMPAT

#endif
