# Uptime Monitor

Monitor the uptime of your servers and HTTP services with a beautiful dashboard, real-time graphs, and drag-and-drop reordering. Built with Go, Alpine.js, TailwindCSS, and Chart.js.

## Features
- Scheduled checks for HTTP, TCP, and Ping services
- Modern dark UI with responsive design
- Real-time uptime graphs for each service
- Add and delete services from the dashboard
- Drag-and-drop to reorder services (order is saved)
- Lightweight and production-ready (Docker support)

## Getting Started

### Prerequisites
- Go 1.20+
- SQLite (included by default)

### Local Development
```sh
go mod tidy
go run .
```
Visit [http://localhost:8080](http://localhost:8080) in your browser.

### Docker
Build and run the app in a production container:
```sh
docker build -t uptime-monitor .
docker run -p 8080:8080 uptime-monitor
```

## Usage
- Add a service by name, type (HTTP, TCP, Ping), and address.
- View uptime history and status for each service.
- Drag cards to reorder; click the red X to delete.

## Tech Stack
- Go (backend, scheduler, API)
- Alpine.js (frontend interactivity)
- TailwindCSS (styling)
- Chart.js (graphs)
- SQLite (storage)

---
Built with ❤️ by DarinDev1000 and GPT-4.1
