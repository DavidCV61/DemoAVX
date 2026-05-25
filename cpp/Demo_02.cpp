#include "Demo_02.h"
#include "AlignedMem.h"
#include <chrono>
#include <fstream>
#include <iomanip>
#include <iostream>
#include <sstream>
#include <string>
#include <vector>

using namespace std;

static void CalcLS(const string &csvFile, const string &jsonFile);

int main(int argc, char *argv[]) {
  if (argc != 3) {
    cerr << "Uso: " << argv[0]
         << " <archivo_entrada.csv> <archivo_salida.json>\n";
    return 1;
  }
  try {
    CalcLS(argv[1], argv[2]);
  } catch (exception &ex) {
    cout << "Ch05_01 exception: " << ex.what() << '\n';
    return 1;
  }
  return 0;
}

void CalcLS(const string &csvFile, const string &jsonFile) {
  vector<double> x_vals, y_vals;
  ifstream fin(csvFile);
  if (!fin.is_open())
    throw runtime_error("No se pudo abrir el archivo CSV :c ");

  string line;
  while (getline(fin, line)) {
    if (line.empty())
      continue;
    istringstream ss(line);
    double x, y;
    char comma;
    if (ss >> x >> comma >> y && comma == ',') {
      x_vals.push_back(x);
      y_vals.push_back(y);
    } else {
      cerr << "Línea ignorada: " << line << '\n';
    }
  }
  fin.close();

  size_t n = x_vals.size();
  if (n < 2)
    throw runtime_error("Se necesitan al menos 2 pares de datos.");

  // Memoria alineada para AVX2
  AlignedArray<double> x_aa(n, c_Alignment);
  AlignedArray<double> y_aa(n, c_Alignment);
  double *x = x_aa.Data();
  double *y = y_aa.Data();
  for (size_t i = 0; i < n; ++i) {
    x[i] = x_vals[i];
    y[i] = y_vals[i];
  }

  const size_t num_iter = 500;
  vector<double> times_cpp(num_iter);
  vector<double> times_avx(num_iter);

  // Benchmark C++
  for (size_t i = 0; i < num_iter; ++i) {
    double m, b;
    auto start = chrono::steady_clock::now();
    CalcLeastSquares_Cpp(&m, &b, x, y, n);
    auto end = chrono::steady_clock::now();
    times_cpp[i] = chrono::duration<double, micro>(end - start).count();
  }

  // Benchmark AVX2
  for (size_t i = 0; i < num_iter; ++i) {
    double m, b;
    auto start = chrono::steady_clock::now();
    CalcLeastSquares_Iavx2(&m, &b, x, y, n);
    auto end = chrono::steady_clock::now();
    times_avx[i] = chrono::duration<double, micro>(end - start).count();
  }

  string benchFile = "benchmark_ls.csv";
  ofstream bf(benchFile);
  if (bf.is_open()) {
    for (size_t i = 0; i < num_iter; ++i) {
      bf << times_cpp[i] << "," << times_avx[i] << "\n";
    }
    bf.close();
    cout << "Benchmark guardado en " << benchFile << "\n";
  } else {
    cerr << "No se pudo guardar " << benchFile << "\n";
  }

  double m, b;
  CalcLeastSquares_Iavx2(&m, &b, x, y, n);

  vector<double> fitted(n);
  for (size_t i = 0; i < n; ++i) {
    fitted[i] = m * x[i] + b;
  }

  // Calcular R^2
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
  ofstream fout(jsonFile);
  if (!fout.is_open())
    throw runtime_error("No se pudo crear el archivo JSON: " + jsonFile);

  fout << fixed << setprecision(12);
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
  cout << fixed << setprecision(8);
  cout << "\nRegresión lineal AVX2\n";
  cout << "  slope     : " << m << '\n';
  cout << "  intercept : " << b << '\n';
  cout << "  R²        : " << r2 << '\n';
  cout << "  JSON guardado en: " << jsonFile << '\n';
}
