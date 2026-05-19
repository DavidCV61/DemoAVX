package main

import (
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	uploadDir   = "../uploads"
	cppBinary   = "../Ch04_06" // binario para umbralización de imágenes
	cppLSBinary = "../Ch05_01" // binario para regresión lineal (con benchmark fijo)
)

func main() {
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		log.Fatal("No se pudo crear uploads:", err)
	}
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", fs)
	http.HandleFunc("/upload", uploadHandler)
	http.HandleFunc("/predict", predictHandler)
	http.HandleFunc("/cpu", cpuHandler)

	port := ":8080"
	fmt.Printf("🚀 Servidor iniciado en http://localhost%s\n", port)
	log.Fatal(http.ListenAndServe(port, nil))
}

// ---------- Procesamiento de imágenes (umbralización) ----------
func uploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}
	file, _, err := r.FormFile("image")
	if err != nil {
		sendError(w, "No se pudo leer la imagen: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	imagePath := filepath.Join(uploadDir, "ImageA.png")
	outFile, err := os.Create(imagePath)
	if err != nil {
		sendError(w, "Error al guardar imagen: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer outFile.Close()
	if _, err := io.Copy(outFile, file); err != nil {
		sendError(w, "Error al escribir imagen: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Ejecutar binario de imágenes (genera máscaras y benchmark.csv)
	cmd := exec.Command(cppBinary)
	cmd.Dir = uploadDir
	output, err := cmd.CombinedOutput()
	log.Printf("Salida del binario de imágenes:\n%s", output)
	if err != nil {
		sendError(w, fmt.Sprintf("Error ejecutando benchmark: %v\n%s", err, output), http.StatusInternalServerError)
		return
	}

	// Leer CSV de benchmark (benchmark.csv)
	csvPath := filepath.Join(uploadDir, "benchmark.csv")
	if _, err := os.Stat(csvPath); os.IsNotExist(err) {
		sendError(w, "No se encontró benchmark.csv", http.StatusInternalServerError)
		return
	}
	cppTimes, avxTimes, err := readBenchmarkCSV(csvPath)
	if err != nil {
		log.Printf("Error leyendo benchmark.csv: %v", err)
		cppTimes = []float64{}
		avxTimes = []float64{}
	}
	stats := computeStatsFromArrays(cppTimes, avxTimes)

	// Leer máscaras generadas
	maskCppPath := filepath.Join(uploadDir, "Ch04_06_ProcessImage_Mask0.png")
	maskAvxPath := filepath.Join(uploadDir, "Ch04_06_ProcessImage_Mask1.png")
	maskCppBase64, err := imageToBase64(maskCppPath)
	if err != nil {
		sendError(w, "Error leyendo máscara C++: "+err.Error(), http.StatusInternalServerError)
		return
	}
	maskAvxBase64, err := imageToBase64(maskAvxPath)
	if err != nil {
		sendError(w, "Error leyendo máscara AVX2: "+err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"images": map[string]string{
			"mask_cpp": maskCppBase64,
			"mask_avx": maskAvxBase64,
		},
		"stats":    stats,
		"cppTimes": cppTimes,
		"avxTimes": avxTimes,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ---------- Predicción de acciones (regresión AR(1) con selector de período) ----------
func predictHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Symbol string `json:"symbol"`
		Days   int    `json:"days"` // días hacia atrás, 0 = máximo histórico
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, "JSON inválido: "+err.Error(), http.StatusBadRequest)
		return
	}
	symbol := strings.ToUpper(strings.TrimSpace(req.Symbol))
	if symbol == "" {
		sendError(w, "Símbolo no proporcionado", http.StatusBadRequest)
		return
	}
	days := req.Days
	if days < 0 {
		days = 365 // valor por defecto seguro
	}

	// 1. Descargar datos históricos con el período seleccionado
	prices, err := fetchYahooPrices(symbol, days)
	if err != nil {
		sendError(w, "Error descargando datos: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if len(prices) < 2 {
		sendError(w, fmt.Sprintf("No hay suficientes datos para regresión (solo %d puntos)", len(prices)), http.StatusBadRequest)
		return
	}
	dataPoints := len(prices)

	// 2. Crear directorio temporal
	tempDir := filepath.Join(uploadDir, "temp_stock")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		sendError(w, "No se pudo crear directorio temporal: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tempDir)

	// 3. Construir CSV con pares (Close(t), Close(t+1))
	csvBase := symbol + ".csv"
	jsonBase := symbol + ".json"
	csvPath := filepath.Join(tempDir, csvBase)
	jsonPath := filepath.Join(tempDir, jsonBase)

	f, err := os.Create(csvPath)
	if err != nil {
		sendError(w, "Error creando CSV: "+err.Error(), http.StatusInternalServerError)
		return
	}
	for i := 0; i < len(prices)-1; i++ {
		fmt.Fprintf(f, "%.6f,%.6f\n", prices[i], prices[i+1])
	}
	f.Close()

	// 4. Ejecutar binario Ch05_01 (genera JSON y benchmark_ls.csv)
	absBin, err := filepath.Abs(cppLSBinary)
	if err != nil {
		sendError(w, "Error resolviendo ruta del binario: "+err.Error(), http.StatusInternalServerError)
		return
	}
	cmd := exec.Command(absBin, csvBase, jsonBase)
	cmd.Dir = tempDir
	output, err := cmd.CombinedOutput()
	log.Printf("Salida de Ch05_01 para %s:\n%s", symbol, output)
	if err != nil {
		sendError(w, fmt.Sprintf("Error ejecutando regresión: %v\n%s", err, output), http.StatusInternalServerError)
		return
	}

	// 5. Leer JSON de predicción
	jsonData, err := os.ReadFile(jsonPath)
	if err != nil {
		sendError(w, "No se pudo leer el JSON de resultados: "+err.Error(), http.StatusInternalServerError)
		return
	}
	var regResult map[string]interface{}
	if err := json.Unmarshal(jsonData, &regResult); err != nil {
		sendError(w, "Error parseando JSON: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 6. Leer CSV de benchmark de regresión (benchmark_ls.csv)
	benchPath := filepath.Join(tempDir, "benchmark_ls.csv")
	var cppTimesReg, avxTimesReg []float64
	if _, err := os.Stat(benchPath); err == nil {
		cppTimesReg, avxTimesReg, err = readBenchmarkCSV(benchPath)
		if err != nil {
			log.Printf("Error leyendo benchmark_ls.csv: %v", err)
		}
	} else {
		log.Printf("No se encontró benchmark_ls.csv en %s", benchPath)
	}

	// 7. Calcular predicción para el día siguiente
	actualRaw, ok := regResult["actualPrices"].([]interface{})
	if !ok || len(actualRaw) == 0 {
		sendError(w, "Formato de resultado inválido", http.StatusInternalServerError)
		return
	}
	actualPrices := make([]float64, len(actualRaw))
	for i, v := range actualRaw {
		actualPrices[i] = v.(float64)
	}
	lastPrice := actualPrices[len(actualPrices)-1]
	slope, _ := regResult["slope"].(float64)
	intercept, _ := regResult["intercept"].(float64)
	prediction := intercept + slope*lastPrice
	rSquared, _ := regResult["rSquared"].(float64)

	// 8. Respuesta al frontend (incluyendo dataPoints)
	response := map[string]interface{}{
		"prediction":     prediction,
		"slope":          slope,
		"intercept":      intercept,
		"rSquared":       rSquared,
		"actualPrices":   actualPrices,
		"fittedPrices":   regResult["fittedPrices"],
		"cppTimesReg":    cppTimesReg,
		"avxTimesReg":    avxTimesReg,
		"dataPoints":     dataPoints,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// readBenchmarkCSV lee CSV de dos columnas (C++, AVX2) y devuelve slices de float64.
func readBenchmarkCSV(filePath string) ([]float64, []float64, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, nil, err
	}
	var cpp, avx []float64
	for _, row := range records {
		if len(row) < 2 {
			continue
		}
		c, err1 := strconv.ParseFloat(strings.TrimSpace(row[0]), 64)
		a, err2 := strconv.ParseFloat(strings.TrimSpace(row[1]), 64)
		if err1 == nil && err2 == nil {
			cpp = append(cpp, c)
			avx = append(avx, a)
		}
	}
	if len(cpp) == 0 {
		return nil, nil, fmt.Errorf("no se encontraron datos numéricos en %s", filePath)
	}
	return cpp, avx, nil
}

// computeStatsFromArrays calcula media, desviación, min, max a partir de los arrays de tiempos.
func computeStatsFromArrays(cppTimes, avxTimes []float64) map[string]interface{} {
	if len(cppTimes) == 0 || len(avxTimes) == 0 {
		return map[string]interface{}{
			"cpp": map[string]float64{"mean": 0, "std": 0, "min": 0, "max": 0},
			"avx": map[string]float64{"mean": 0, "std": 0, "min": 0, "max": 0},
		}
	}
	// C++
	sumCpp := 0.0
	minCpp, maxCpp := cppTimes[0], cppTimes[0]
	for _, v := range cppTimes {
		sumCpp += v
		if v < minCpp {
			minCpp = v
		}
		if v > maxCpp {
			maxCpp = v
		}
	}
	meanCpp := sumCpp / float64(len(cppTimes))
	var varCpp float64
	for _, v := range cppTimes {
		diff := v - meanCpp
		varCpp += diff * diff
	}
	stdCpp := varCpp / float64(len(cppTimes))

	// AVX2
	sumAvx := 0.0
	minAvx, maxAvx := avxTimes[0], avxTimes[0]
	for _, v := range avxTimes {
		sumAvx += v
		if v < minAvx {
			minAvx = v
		}
		if v > maxAvx {
			maxAvx = v
		}
	}
	meanAvx := sumAvx / float64(len(avxTimes))
	var varAvx float64
	for _, v := range avxTimes {
		diff := v - meanAvx
		varAvx += diff * diff
	}
	stdAvx := varAvx / float64(len(avxTimes))

	return map[string]interface{}{
		"cpp": map[string]float64{"mean": meanCpp, "std": stdCpp, "min": minCpp, "max": maxCpp},
		"avx": map[string]float64{"mean": meanAvx, "std": stdAvx, "min": minAvx, "max": maxAvx},
	}
}

// fetchYahooPrices descarga precios de cierre ajustados.
// days > 0: últimos N días hábiles (aproximado)
// days == 0: máximo histórico posible (desde 1980)
func fetchYahooPrices(symbol string, days int) ([]float64, error) {
	var period1 int64
	if days <= 0 {
		// Máximo histórico: desde 1980-01-01
		start := time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)
		period1 = start.Unix()
	} else {
		// Se añade un margen de 20 días para compensar fines de semana y feriados
		start := time.Now().AddDate(0, 0, -days-20)
		period1 = start.Unix()
	}
	period2 := time.Now().Unix()
	url := fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s?period1=%d&period2=%d&interval=1d", symbol, period1, period2)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var yf struct {
		Chart struct {
			Result []struct {
				Indicators struct {
					Quote []struct {
						Close []*float64 `json:"close"`
					} `json:"quote"`
				} `json:"indicators"`
			} `json:"result"`
		} `json:"chart"`
	}
	if err := json.Unmarshal(body, &yf); err != nil {
		return nil, err
	}
	if len(yf.Chart.Result) == 0 {
		return nil, fmt.Errorf("no hay datos para %s", symbol)
	}
	closes := yf.Chart.Result[0].Indicators.Quote[0].Close
	var prices []float64
	for _, c := range closes {
		if c != nil {
			prices = append(prices, *c)
		}
	}
	if len(prices) < 2 {
		return nil, fmt.Errorf("solo %d precios válidos (necesarios 2+)", len(prices))
	}
	// Si se pidió un número específico de días, limitar a los últimos 'days'
	if days > 0 && len(prices) > days {
		prices = prices[len(prices)-days:]
	}
	return prices, nil
}

// imageToBase64 convierte una imagen a base64.
func imageToBase64(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

// sendError envía mensaje de error en JSON.
func sendError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// cpuHandler devuelve el modelo del procesador.
func cpuHandler(w http.ResponseWriter, r *http.Request) {
	model := getCPUModel()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"model": model})
}

// getCPUModel obtiene el nombre del procesador (Linux/macOS).
func getCPUModel() string {
	if runtime.GOOS == "linux" {
		data, err := os.ReadFile("/proc/cpuinfo")
		if err == nil {
			lines := strings.Split(string(data), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "model name") {
					parts := strings.SplitN(line, ":", 2)
					if len(parts) == 2 {
						return strings.TrimSpace(parts[1])
					}
				}
			}
		}
	} else if runtime.GOOS == "darwin" {
		out, err := exec.Command("sysctl", "-n", "machdep.cpu.brand_string").Output()
		if err == nil {
			return strings.TrimSpace(string(out))
		}
	}
	return "CPU con soporte AVX2"
}
