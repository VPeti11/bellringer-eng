#include "pico/stdlib.h"
#include <string>

int main() {
    stdio_init_all();

    const uint RELAY_PIN = 1;
    gpio_init(RELAY_PIN);
    gpio_set_dir(RELAY_PIN, GPIO_OUT);

    // Relay OFF at startup (active‑LOW → HIGH = OFF)
    gpio_put(RELAY_PIN, 1);

    std::string buffer;

    while (true) {
        int ch = getchar_timeout_us(0);

        if (ch == PICO_ERROR_TIMEOUT) {
            continue;
        }

        if (ch == '\n' || ch == '\r') {
            if (buffer == "HIGH") {
                // HIGH command → relay ON
                gpio_put(RELAY_PIN, 0);
                printf("OK HIGH\n");
            } else if (buffer == "LOW") {
                // LOW command → relay OFF
                gpio_put(RELAY_PIN, 1);
                printf("OK LOW\n");
            } else if (!buffer.empty()) {
                printf("ERR UNKNOWN\n");
            }
            buffer.clear();
        } else {
            buffer.push_back((char)ch);
        }
    }
}
