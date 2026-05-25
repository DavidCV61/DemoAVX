# ================= ETAPA 1: Compilar binarios C++ =================
FROM ubuntu:22.04 AS cpp-builder

RUN apt-get update && apt-get install -y \
    g++ \
    make \
    libpng-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /build/cpp
COPY cpp/ .

# Compilar demo_01 (umbralización de imágenes)
RUN g++ -std=c++17 -O3 -mavx2 -mfma -static-libstdc++ \
    Demo_01.cpp \
    Demo_01_bm.cpp \
    Demo_01_funciones.cpp \
    Demo_01_misc.cpp \
    -pthread -lpng -o demo_01

# Compilar demo_02 (regresión lineal)
RUN g++ -std=c++17 -O3 -mavx2 -mfma -static-libstdc++ \
    Demo_02.cpp \
    Demo_02_funciones.cpp \
    Demo_02_misc.cpp \
    -lm -o demo_02

# ================= ETAPA 2: Compilar servidor Go =================
FROM golang:1.21-alpine AS go-builder

WORKDIR /build/backend
COPY backend/ .

RUN go mod init backend 2>/dev/null || true && \
    go mod tidy && \
    CGO_ENABLED=0 GOOS=linux go build -o server .

# ================= ETAPA 3: Imagen final =================
FROM ubuntu:22.04

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    libpng16-16 \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copiar binarios C++ (enlace estático de libstdc++)
COPY --from=cpp-builder /build/cpp/demo_01 ./
COPY --from=cpp-builder /build/cpp/demo_02 ./
RUN chmod +x demo_01 demo_02

# Copiar servidor Go y archivos estáticos
COPY --from=go-builder /build/backend/server ./backend/
COPY backend/static ./backend/static/

# Crear directorio de uploads (será ../uploads desde backend)
RUN mkdir -p /app/uploads && chmod 777 /app/uploads

# Usuario no root
RUN useradd -m -u 1000 appuser && chown -R appuser:appuser /app
USER appuser

WORKDIR /app/backend
EXPOSE 8080
CMD ["./server"]
