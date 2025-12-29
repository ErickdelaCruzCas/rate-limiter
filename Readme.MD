# Rate Limiter Cookbook (Bazel-first)

Este repositorio es un **cookbook técnico**, incremental y honesto, cuyo objetivo es aprender **Bazel** y **system design** usando un **rate limiter distribuido** como hilo conductor.

No es un producto final.
No es una librería lista para producción.
No intenta impresionar con complejidad.

Intenta **enseñar cómo se piensan y se construyen sistemas**, y por qué Bazel cambia la forma de estructurar un proyecto serio.

---

## Qué problema resuelve un rate limiter (en la práctica)

Un rate limiter decide si una petición se **acepta** o se **rechaza** según una política de tráfico.

Se usa para:

* Proteger servicios downstream (bases de datos, terceros, colas)
* Evitar abuso en APIs públicas
* Mantener SLOs bajo carga
* Implementar fairness (por usuario, token, IP, tenant)

Hasta aquí, trivial.

Lo interesante aparece cuando el sistema es **distribuido**:

* ¿Dónde vive el estado?
* ¿Qué pasa si el estado no es consistente?
* ¿Cuánta precisión necesito realmente?
* ¿Qué latencia estoy dispuesto a pagar?

Este repo se centra en **esas preguntas**, no en definiciones de libro.

---

## Por qué existe este repositorio

La mayoría de ejemplos de rate limiting que circulan por ahí:

* Son *toy examples* sin edge cases
* Mezclan algoritmo, red, framework y reloj real en la misma clase
* No son reproducibles
* No permiten razonar sobre trade-offs

Además, casi ninguno sirve para **aprender un build system moderno**.

Aquí el rate limiter es la excusa.
El verdadero protagonista es **Bazel**.

---

## Por qué Bazel (y no Maven / Gradle)

Maven funciona bien para proyectos Java tradicionales.

Pero empieza a romperse cuando quieres:

* Monorepo real (no varios repos pegados)
* Código en varios lenguajes (Java, Go, proto)
* Builds herméticos y reproducibles
* Cacheo agresivo a nivel de target
* Tests y benchmarks como ciudadanos de primera

En Maven, esto suele acabar en:

* Perfiles
* Scripts externos
* Documentación implícita
* Builds lentos y frágiles

Bazel fuerza otra mentalidad:

* Todo es un target
* Las dependencias son explícitas
* El build es un grafo
* Si no cambia el input, no se ejecuta nada

Este repositorio está diseñado para **mostrar eso en la práctica**, no explicarlo en abstracto.

---

## Qué NO intenta resolver este proyecto

Conviene dejarlo claro desde el principio:

* No es un servicio completo listo para producción
* No implementa alta disponibilidad real
* No incluye sharding avanzado ni replicación
* No integra Redis, DynamoDB ni otros KV distribuidos (al principio)
* No resuelve autenticación, billing ni planes de suscripción

Todo eso **vendría después**, si el objetivo fuera un producto.

Aquí el objetivo es **entender las piezas**, no esconderlas.

---

## Filosofía del proyecto

Principios no negociables:

* Bazel es el protagonista, no Java
* Cada carpeta tiene su `BUILD.bazel`
* Nada de scripts mágicos fuera de Bazel
* Código simple, pero correcto
* Trade-offs explícitos, no implícitos
* Tests y benchmarks son targets
* Todo debe ser incremental y cacheable

Si algo es complejo, tiene que estar justificado.

---

## Estructura del monorepo

Visión de alto nivel:

```
rate-limiter-cookbook/
├── MODULE.bazel
├── WORKSPACE
├── README.md
├── docs/
│   └── design-notes.md
├── proto/
│   └── ratelimit.proto
├── core/
│   ├── clock/
│   ├── model/
│   └── algorithms/
├── java/
│   ├── engine/
│   ├── grpc/
│   ├── tests/
│   └── benchmarks/
├── go/
│   ├── engine/
│   ├── tests/
│   └── benchmarks/
├── load/
│   └── traffic_generator.go
└── infra/
    └── docker/
```

Cada capa añade **una dimensión nueva de complejidad**, nunca varias a la vez.

---

## Fases del proyecto

El proyecto se construye **por fases**, de forma deliberada:

### Fase 0 — README y mentalidad

Antes de escribir código:

* Definir qué problema resolvemos
* Definir qué NO resolvemos
* Alinear expectativas

### Fase 1 — Core puro (sin I/O)

* Algoritmos deterministas
* Sin red, sin threads, sin clocks reales
* Reloj inyectable
* Tests exhaustivos

Algoritmos:

* Token Bucket
* Fixed Window
* Sliding Window
* Sliding Window Counter

### Fase 2 — Tests como ciudadanos de primera

* Tests unitarios por algoritmo
* Edge cases
* Regresión temporal con reloj mockeado

### Fase 3 — Java Engine

* Wrapper thread-safe
* In-memory
* Sin frameworks
* Pensado para alto throughput

### Fase 4 — gRPC API

* Contrato en proto
* Codegen con Bazel
* Server Java

### Fase 5 — Implementación en Go

* Mismo contrato
* Diferencias de diseño Java vs Go
* Bazel como orquestador polyglot

### Fase 6 — Benchmarks

* JMH en Java
* testing/benchmark en Go
* Benchmarks como targets

### Fase 7 — Load testing

* Generador de tráfico propio
* Reproducible
* Sin herramientas externas

---

## Cómo usar este repositorio

Ejecutar tests del core:

```bash
bazel test //core/...
```

Ejecutar benchmarks (fases posteriores):

```bash
bazel run //java/benchmarks:token_bucket
bazel test //go/benchmarks:sliding_window
```

Bazel se encarga de ejecutar **solo lo que cambia**.

---

## Lessons learned (vivo)

Esta sección se irá rellenando con conclusiones reales:

* Bazel premia targets pequeños y bien definidos
* Sliding window es más preciso, pero más caro
* En distribuido, la perfección suele ser innecesaria
* La mayoría de sistemas necesitan aproximaciones razonables, no exactitud matemática

---

## Licencia

MIT (o la que prefieras).
