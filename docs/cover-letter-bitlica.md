# Cover letter — Bitlica · Backend Vibe Engineer (Golang)

Привет, команда Bitlica.

Подаюсь на позицию Backend Vibe Engineer (Golang). Хочу прямо обозначить, по чему совпадаем, а где у меня ramp — потому что в названии стоит «Vibe», и именно эта часть, кажется, и есть где я могу быть полезен.

**Что приношу:**

- Два года production-бэкенда на Wentmarket ([wentmarket.ru](https://wentmarket.ru)) — B2B HVAC-платформа, которую я собрал end-to-end с Claude Code. 17 000+ полиномиальных кривых вентиляторов в Postgres, подбор по рабочей точке за p95 < 100 мс, реальные платежи через Yookassa, реальные заказы через CDEK. Order pipeline, soft-delete, идемпотентные платежи, headless Bitrix24, sync с 1С — всё в продакшне.
- Рабочий spec-first процесс с AI-coding агентами. PRD → architecture → per-story acceptance criteria → код → тесты → деплой. Не «дайте Cursor доавтокомплитнуть», а настоящий цикл, где спека гейтит реализацию, а AI — мультипликатор продуктивности на каждом этапе. BMad-style.
- Шесть лет HVAC-инжиниринга до веба — пригождается, когда домен технический (а в бэкенде он часто такой).

**Чего пока нет:**

- Production-Go. Primary stack — TypeScript/Node. Чтобы закрыть гэп, собрал реальный Go-сервис в том же spec-driven стиле: [github.com/goncharovart/fan-selector-api](https://github.com/goncharovart/fan-selector-api). Distroless образ, embedded SQL-миграции, OTel traces, integration-тесты на testcontainers-go, GitHub Actions CI, спеки в `/specs`. Бенчмарки: Horner-eval 0.65 нс, bisection 53 нс, полный Evaluate 50 кандидатов 3.7 мкс — у p95-бюджета 100 мс остаётся 99 мс запаса. Это вынос движка подбора из Wentmarket в чистый микросервис — маленький проект, но каждый слой того, что вы перечислили в вакансии, реально лежит в репо и его можно поковырять.
- Хендс-он GKE специально. Cloud Run в managed-конфигурации трогал, Compute Engine для self-hosted — конечно. Примитивы (поды, сервисы, configmaps, секреты) у меня в голове из self-hosted сетапов; продакшн-операции GKE нагоню быстрее всего в деле.

**Бонус — связанный Python pet:** [github.com/goncharovart/fan-curve-fitter](https://github.com/goncharovart/fan-curve-fitter). CLI на Typer + numpy + pydantic, который фитит полиномы из CSV-замеров и отдаёт JSON прямо в формате `fan-selector-api`. Получился маленький end-to-end pipeline: лабораторные данные → Python → JSON → Go-сервис. Horner-эвалюация в Python и Go синхронизирована, чтобы numerical output совпадал.

**Почему эта роль:**

«Vibe Engineer» — это и есть то, что я уже делаю. Большая часть «опытных senior»-кандидатов, которые к вам придут, никогда не отгружали реальный продакшн с AI-агентами в loop'е. У меня это получилось. Wentmarket — артефакт; fan-selector-api + fan-curve-fitter — доказательство того, что повторю это на вашем стеке.

Был бы рад 30-минутному звонку, чтобы пройти по обоим репам с вами и узнать, как «AI-native workflow» Bitlica устроен на практике.

Открыт к релокации (Польша / Грузия / Беларусь / другое). Готов начать сразу.

Спасибо, что прочитали,
Артур Гончаров
[@gonartur](https://t.me/gonartur) · +7 (995) 376-31-73
