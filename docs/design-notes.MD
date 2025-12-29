# Design Notes — Rate Limiter Cookbook

## Objetivo del core (Fase 1)
Separar el **algoritmo** de:
- tiempo real
- I/O
- threads
- persistencia

El core debe ser:
- determinista
- testeable con reloj mockeado
- intercambiable por estrategia (TokenBucket, FixedWindow, SlidingWindow, SlidingWindowCounter)

## API conceptual mínima

- Input: (key, cost, now)
- Output: allow/deny + metadatos (remaining, resetAt, retryAfter, etc.)

Decisiones:
- El reloj entra por interfaz (`Clock`) para tests.
- Cada algoritmo vive en su target Bazel.
- Los tests son targets por algoritmo.

## Trade-offs que vamos a documentar
- Token bucket: burst-friendly, estado simple, precisa refill.
- Fixed window: simple, pero “boundary problem”.
- Sliding window: preciso, más estado y coste.
- Sliding window counter: aproximación práctica, mejor que fixed sin el coste completo.
