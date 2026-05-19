//------------------------------------------------
//               Ch05_01.cpp (con benchmark fijo)
//------------------------------------------------

#include "Ch05_01.h"
#include "AlignedMem.h"
#include <chrono>
#include <cmath>
#include <fstream>
#include <iomanip>
#include <iostream>
#include <sstream>
#include <string>
#include <vector>

static void CalcLS(const std::string &csvFile, const std::string &jsonFile);

int main(int argc, char *argv[]) {
  if (argc != 3) {
    std::cerr << "Uso: " << argv[0]
              << " <archivo_entrada.csv> <archivo_salida.json>\n";
    return 1;
  }
  try {
    CalcLS(argv[1], argv[2]);
  } catch (std::exception &ex) {
    std::cout << "Ch05_01 exception: " << ex.what() << '\n';
    return 1;
  }
  return 0;
}

void CalcLS(const std::string &csvFile, const std::string &jsonFile) {
  // 1. Leer CSV con dos columnas: x = Close(t), y = Close(t+1)
  std::vector<double> x_vals, y_vals;
  std::ifstream fin(csvFile);
  if (!fin.is_open())
    throw std::runtime_error("No se pudo abrir el archivo CSV: " + csvFile);

  std::string line;
  while (std::getline(fin, line)) {
    if (line.empty())
      continue;
    std::istringstream ss(line);
    double x, y;
    char comma;
    if (ss >> x >> comma >> y && comma == ',') {
      x_vals.push_back(x);
      y_vals.push_back(y);
    } else {
      std::cerr << "Línea ignorada: " << line << '\n';
    }
  }
  fin.close();

  size_t n = x_vals.size();
  if (n < 2)
    throw std::runtime_error("Se necesitan al menos 2 pares de datos.");

  // Memoria alineada para AVX2
  AlignedArray<double> x_aa(n, c_Alignment);
  AlignedArray<double> y_aa(n, c_Alignment);
  double *x = x_aa.Data();
  double *y = y_aa.Data();
  for (size_t i = 0; i < n; ++i) {
    x[i] = x_vals[i];
    y[i] = y_vals[i];
  }

  // ---------- Benchmark (500 iteraciones para C++ y AVX2) ----------
  const size_t num_iter = 500;
  std::vector<double> times_cpp(num_iter);
  std::vector<double> times_avx(num_iter);

  // Benchmark C++
  for (size_t i = 0; i < num_iter; ++i) {
    double m, b;
    auto start = std::chrono::steady_clock::now();
    CalcLeastSquares_Cpp(&m, &b, x, y, n);
    auto end = std::chrono::steady_clock::now();
    times_cpp[i] =
        std::chrono::duration<double, std::micro>(end - start).count();
  }

  // Benchmark AVX2
  for (size_t i = 0; i < num_iter; ++i) {
    double m, b;
    auto start = std::chrono::steady_clock::now();
    CalcLeastSquares_Iavx2(&m, &b, x, y, n);
    auto end = std::chrono::steady_clock::now();
    times_avx[i] =
        std::chrono::duration<double, std::micro>(end - start).count();
  }

  // Guardar benchmark en CSV
  std::string benchFile = "benchmark_ls.csv";
  std::ofstream bf(benchFile);
  if (bf.is_open()) {
    for (size_t i = 0; i < num_iter; ++i) {
      bf << times_cpp[i] << "," << times_avx[i] << "\n";
    }
    bf.close();
    std::cout << "Benchmark guardado en " << benchFile << "\n";
  } else {
    std::cerr << "No se pudo guardar " << benchFile << "\n";
  }

  // ---------- Cálculo final (usando AVX2) para la predicción ----------
  double m, b;
  CalcLeastSquares_Iavx2(&m, &b, x, y, n);

  // Valores ajustados
  std::vector<double> fitted(n);
  for (size_t i = 0; i < n; ++i) {
    fitted[i] = m * x[i] + b;
  }

  // Calcular R²
  double sum_y = 0.0;
  for (size_t i = 0; i < n; ++i)
    sum_y += y[i];
  double mean_y = sum_y / n;
  double ss_res = 0.0, ss_tot = 0.0;
  for (size_t i = 0; i < n; ++i) {
    ss_res += (y[i] - fitted[i]) * (y[i] - fitted[i]);
    ss_tot += (y[i] - mean_y) * (y[i] - mean_y);
  }
  double r2 = (ss_tot > 0.0) ? 1.0 - ss_res / ss_tot : 0.0;

  // Escribir archivo JSON
  std::ofstream fout(jsonFile);
  if (!fout.is_open())
    throw std::runtime_error("No se pudo crear el archivo JSON: " + jsonFile);

  fout << std::fixed << std::setprecision(12);
  fout << "{\n";
  fout << "  \"slope\": " << m << ",\n";
  fout << "  \"intercept\": " << b << ",\n";
  fout << "  \"rSquared\": " << r2 << ",\n";
  fout << "  \"actualPrices\": [";
  for (size_t i = 0; i < n; ++i) {
    fout << y[i] << (i + 1 < n ? ", " : "");
  }
  fout << "],\n";
  fout << "  \"fittedPrices\": [";
  for (size_t i = 0; i < n; ++i) {
    fout << fitted[i] << (i + 1 < n ? ", " : "");
  }
  fout << "]\n";
  fout << "}\n";
  fout.close();

  // Consola
  std::cout << std::fixed << std::setprecision(8);
  std::cout << "\nRegresión lineal AVX2\n";
  std::cout << "  slope     : " << m << '\n';
  std::cout << "  intercept : " << b << '\n';
  std::cout << "  R²        : " << r2 << '\n';
  std::cout << "  JSON guardado en: " << jsonFile << '\n';
}
