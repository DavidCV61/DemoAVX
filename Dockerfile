# ================= ETAPA 1: Compilar binarios C++ =================
FROM ubuntu:22.04 AS cpp-builder

RUN apt-get update && apt-get install -y \
    g++ \
    make \
    libpng-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /build/cpp
COPY cpp/ .

# Compilar Ch04_06 (enlace estático de libstdc++)
RUN g++ -std=c++17 -O3 -mavx2 -mfma -static-libstdc++ \
    Ch04_06.cpp \
    Ch04_06_bm.cpp \
    Ch04_06_fcpp.cpp \
    Ch04_06_misc.cpp \
    -pthread -lpng -o Ch04_06

# Compilar Ch05_01 (enlace estático de libstdc++)
RUN g++ -std=c++17 -O3 -mavx2 -mfma -static-libstdc++ \
    Ch05_01.cpp \
    Ch05_01_fcpp.cpp \
    Ch05_01_misc.cpp \
    -pthread -lpng -o Ch05_01

# ================= ETAPA 2: Compilar servidor Go =================
FROM golang:1.21-alpine AS go-builder

WORKDIR /build/backend
COPY backend/ .

RUN go mod init backend 2>/dev/null || true && \
    go mod tidy && \
    CGO_ENABLED=0 GOOS=linux go build -o server .

# ================= ETAPA 3: Imagen final =================
FROM ubuntu:22.04

# Solo dependencias runtime necesarias (libpng, CA certs)
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    libpng16-16 \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copiar binarios C++ (con libstdc++ estática)
COPY --from=cpp-builder /build/cpp/Ch04_06 ./
COPY --from=cpp-builder /build/cpp/Ch05_01 ./
RUN chmod +x Ch04_06 Ch05_01

# Copiar servidor Go y estáticos
COPY --from=go-builder /build/backend/server ./backend/
COPY backend/static ./backend/static/

# Crear directorio de uploads
RUN mkdir -p /app/uploads && chmod 777 /app/uploads

# Usuario no root
RUN useradd -m -u 1000 appuser && chown -R appuser:appuser /app
USER appuser

WORKDIR /app/backend
EXPOSE 8080
CMD ["./server"]
