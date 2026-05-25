#include <fstream>
#include <iostream>
#include <istream>
#include <sstream>
#include <string>
#include <vector>

using namespace std;

static void CalcLS(const string &csvFile, const string &jsonFile);

int main(int argc, char *argv[]) {

  CalcLS(argv[1], argv[2]);

  return 0;
}

void CalcLS(const string &csvFile, const string &jsonFile) {

  vector<double> x_vals, y_vals;

  ifstream datos(csvFile);
  if (!datos.is_open()) {
    throw runtime_error("No se pudo abrir el archivo :c ");
  }
  string linea;
  while (getline(datos, linea)) {
    if (linea.empty()) {
      continue;
    }

    istringstream ss(linea);

    double x, y;

    char comma;

    if (ss >> x >> comma >> y && comma == ',') {
      x_vals.push_back(x);
      y_vals.push_back(y);
    } else {
      cerr << "Línea ignorada: " << linea << '\n';
    }
  }

  datos.close();
  /*
    for (double x : x_vals) {

      cout << x << endl;
    }

    */

  size_t n = x_vals.size();
  if (n < 2)
    throw runtime_error("Se necesitan al menos 2 pares de datos");

  // Memoria alineada para AVX2
  AlignedArray<double> x_aa(n, c_Alignment);
  AlignedArray<double> y_aa(n, c_Alignment);
  double *x = x_aa.Data();
  double *y = y_aa.Data();
  for (size_t i = 0; i < n; ++i) {
    x[i] = x_vals[i];
    y[i] = y_vals[i];
  }
}
