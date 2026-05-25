import csv
import requests
import time
from datetime import datetime, timedelta

symbol = "INTC"
days = 365

period2 = int(time.time())
start_date = datetime.now() - timedelta(days=days + 20)
period1 = int(start_date.timestamp())

url = f"https://query1.finance.yahoo.com/v8/finance/chart/{
    symbol}?period1={period1}&period2={period2}&interval=1d"
headers = {"User-Agent": "Mozilla/5.0", "Accept": "application/json"}
resp = requests.get(url, headers=headers, timeout=15)
resp.raise_for_status()

data = resp.json()
closes = data["chart"]["result"][0]["indicators"]["quote"][0]["close"]
prices = [float(c) for c in closes if c is not None]

if len(prices) > days:
    prices = prices[-days:]

with open("datos.csv", 'w', newline='') as f:
    writer = csv.writer(f)
    for i in range(len(prices) - 1):
        writer.writerow([f"{prices[i]:.6f}", f"{prices[i+1]:.6f}"])

print(f"Generado datos.csv con {len(prices)} precios")
