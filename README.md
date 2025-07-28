# Logs Distributor

A high-throughput logs distributor with weighted round-robin distribution, retry logic, and failure handling.

## Quick Start

```bash
# Start the service
docker-compose up --build

# In another terminal, test the service:

# Health check
curl http://localhost:8080/api/v1/health

# View stats
curl http://localhost:8080/api/v1/stats

# Submit custom packet
curl -X POST http://localhost:8080/api/v1/logs \
  -H "Content-Type: application/json" \
  -d '[{
    "messages": [
      {
        "level": "ERROR",
        "message": "Database connection failed",
        "source": "api-service"
      }
    ]
  }]'

# Check failed packets
curl http://localhost:8080/api/v1/dead-letter
```

## Alternative: Local Go Build

```bash
make build    # Build binary
make run      # Build and run service  
make clean    # Clean build files
make help     # Show help
```

## Key Features

### 🎯 **Weighted Round-Robin Distribution**
- **Analyzer 1**: 40% of traffic
- **Analyzer 2**: 30% of traffic  
- **Analyzer 3**: 20% of traffic
- **Analyzer 4**: 10% of traffic

Guarantees exact proportional distribution using deterministic algorithm.

### 🔄 **Retry Logic with Exponential Backoff**
```
Attempt 1: Fails → Wait 2s  → Retry
Attempt 2: Fails → Wait 4s  → Retry  
Attempt 3: Fails → Wait 8s  → Retry
Attempt 4: Fails → Save to failed_packets.json
```

### 💾 **Dead Letter File**
Failed packets saved to `failed_packets.json`:
```json
[
  {
    "packet": {
      "id": "abc-123",
      "messages": [...],
      "retry_count": 3
    },
    "final_error": "analyzer crashed during processing",
    "failed_at": "2024-01-01T10:00:15Z"
  }
]
```

### 🏥 **Health Monitoring**
- Automatic health checks every 10 seconds
- Failed analyzers excluded from distribution
- Traffic automatically redistributed to healthy analyzers

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/health` | Service health status |
| GET | `/api/v1/stats` | Detailed statistics |
| GET | `/api/v1/analyzers` | Analyzer information |
| GET | `/api/v1/dead-letter` | Failed packets |
| POST | `/api/v1/logs` | Submit log packets |
| POST | `/api/v1/analyzers/:id/health` | Manual health control |

## API Response Examples

### POST `/api/v1/logs` Response

**Successful submission:**
```json
{
  "total_packets": 3,
  "successful": 2,
  "failed": 1,
  "processed_packets": ["packet-id-abc-123", "packet-id-def-456"]
}
```

### GET `/api/v1/stats` Response

```json
{
  "total_packets_received": 1250,
  "total_messages_routed": 2830,
  "active_analyzers": 4,
  "packet_channel_util_percent": 23.4,
  "result_channel_util_percent": 12.1,
  "retry_channel_util_percent": 0.0,
  "uptime": "5m23.891s",
  "timestamp": "2025-01-25T20:30:45Z",
  "analyzers": {
    "analyzer-a1": {
      "name": "Analyzer A",
      "is_healthy": true,
      "processed_count": 520,
      "error_count": 12
    },
    "analyzer-a2": {
      "name": "Analyzer B", 
      "is_healthy": true,
      "processed_count": 385,
      "error_count": 8
    },
    "analyzer-a3": {
      "name": "Analyzer C",
      "is_healthy": true,
      "processed_count": 245,
      "error_count": 3
    },
    "analyzer-a4": {
      "name": "Analyzer D",
      "is_healthy": false,
      "processed_count": 100,
      "error_count": 45
    }
  }
}
```

### GET `/api/v1/health` Response
```json
{
  "status": "degraded",
  "uptime": "5m23s", 
  "active_analyzers": 2,
  "total_analyzers": 4,
  "timestamp": "2025-01-25T20:30:45Z"
}
```
### How it Works

**1. Packet Submission** 📨
1. **API Layer** receives HTTP request with log packets
2. **PacketValidator** validates format, size, and content
3. **Distributor** accepts packet and tracks for retry
4. **Packet** queued for processing

**2. Load Balancing** ⚖️
1. **LoadBalancer** selects healthy analyzer using weighted round-robin
2. **HealthMonitor** ensures only healthy analyzers are considered
3. **Selected analyzer** receives packet for processing

**3. Packet Processing** ⚙️
1. **PacketProcessor** simulates analysis (configurable processing time)
2. **Random failures** simulate real-world conditions (5% failure rate)
3. **Results** sent back with success/failure status

**4. Retry Logic** 🔄
1. **RetryHandler** tracks failed packets
2. **Exponential backoff** delays retries (2s → 4s → 8s)
3. **Max retries** reached → save to dead letter file
4. **Successful retry** → remove from tracking

**5. State Persistence** 💾
1. **PersistenceManager** saves state every 30 seconds
2. **Gzip compression** for efficient storage
3. **Recovery on restart** restores in-flight packets
4. **Graceful shutdown** saves final state

## File Structure

```
logs-distributor/
├── main.go                           # Service entry point with DI
├── api/handlers.go                   # HTTP API layer
├── config/config.go                  # Configuration constants
├── models/models.go                  # Data structures
└── distributor/
    ├── interfaces/                   # 📝 All abstractions
    │   ├── distributor.go            # Main service interface
    │   ├── load_balancer.go          # Load balancing interface
    │   ├── health_monitor.go         # Health monitoring interface
    │   ├── persistence.go            # Persistence interface
    │   ├── retry_handler.go          # Retry logic interface
    │   ├── packet_processor.go       # Processing interface
    │   └── packet_validator.go       # Validation interface
    ├── implementations/              # 🔧 Concrete implementations
    ├── ├── distributor.go            # Main orchestrator
    │   ├── load_balancer.go          # Weighted round-robin
    │   ├── health_monitor.go         # Health checking
    │   ├── persistence_manager.go    # File-based persistence
    │   ├── retry_handler.go          # Exponential backoff retry
    │   ├── packet_processor.go       # Packet analysis simulation
    │   └── packet_validator.go       # Input validation
    └── tests/                        # 🧪 Comprehensive test suite
        ├── distributor_test.go       # End-to-end functionality
        ├── load_balancer_test.go     # Load balancing logic
        ├── health_monitor_test.go    # Health monitoring
        ├── packet_validator_test.go  # Validation rules
        ├── packet_processor_test.go  # Processing behavior
        ├── retry_handler_test.go     # Retry logic
        └── persistence_manager_test.go # File persistence
```
### **Decisions and Assumptions**
**Channel-Based Architecture**
- **Decision**: In-memory Go channels vs external message queue
- **Rationale**: Optimal for single-instance demo with minimal latency and complexity
- **Trade-off**: Simplicity and performance vs distributed scaling capability

**Retry with Exponential Backoff**
- **Decision**: Progressive delays (2s → 4s → 8s) vs fixed intervals
- **Rationale**: Reduces load on failing analyzers while providing recovery opportunity
- **Implementation**: Goroutine-based delayed retry with cleanup

**State Persistence**
- **Decision**: Periodic snapshots (30s) vs real-time persistence
- **Rationale**: Balance between data safety and performance overhead
- **Recovery**: JSON-based state restoration on restart


**Component-Based Testing**
- **Decision**: One test file per component (load_balancer_test.go, etc.)
- **Rationale**: Clear organization and focused testing
- **Benefit**: Easy to find and maintain tests for specific functionality

**Current Processing Time Assumptions**
- **Distributor**: ~1ms per packet (in-memory routing)
- **Analyzers**: 100-2000ms per message (configurable simulation)
- **Validation**: <1ms per packet (input validation)
- **Persistence**: ~10ms per checkpoint (gzip compression)

### Using JMeter
1. **HTTP Request Configuration:**
   - URL: `http://localhost:8080/api/v1/logs`
   - Method: `POST`
   - Content-Type: `application/json`

2. **Sample Request Body:**
```json
[
  {
    "messages": [
      {
        "level": "INFO",
        "message": "User login successful",
        "source": "auth-service",
        "metadata": {
          "user_id": "user_12345",
          "ip_address": "192.168.1.100",
          "session_id": "sess_abc123"
        }
      },
      {
        "level": "DEBUG",
        "message": "Session token generated",
        "source": "auth-service",
        "metadata": {
          "user_id": "user_12345",
          "token_expiry": "2025-01-25T21:30:00Z"
        }
      },
      {
        "level": "ERROR",
        "message": "Multiple login attempts detected",
        "source": "security-monitor",
        "metadata": {
          "user_id": "user_12345",
          "attempt_count": 5,
          "last_attempt": "2025-01-25T20:25:00Z"
        }
      }
    ]
  }
]
```

3. **Exaple Thread Settings:**
   - Threads: 50-100
   - Ramp-up: 10 seconds
   - Loop Count: 5

### Monitor During Testing

**Start the service with Docker:**
```bash
docker-compose up --build
```

**Monitor in another terminal:**
```bash
# Real-time stats monitoring
watch -n 2 'curl -s http://localhost:8080/api/v1/stats | python -m json.tool'
```

**Key metrics to watch:**
- `packet_channel_util_percent`: Packet processing queue load (0-100%)
- `total_packets_received`: Total throughput (packets processed)
- `active_analyzers`: Number of healthy analyzer services

That's it! Simple, focused, and easy to test. 🎯 
