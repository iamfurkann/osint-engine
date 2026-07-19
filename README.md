# OSINT Engine

Modüler Açık Kaynak İstihbarat (OSINT) Platformu. 
Kişisel araştırmalar için tasarlanmış, plugin tabanlı çekirdek motor, CLI ve Daemon hibrit mimarisini kullanır.

## Başlangıç

Derlemek için:
```bash
make build

#### 5. `cmd/osint/main.go`
Kullanıcının ana etkileşimde bulunacağı CLI aracının (Foreground) giriş noktası. P0 yönergelerine göre şu an sadece çalışır durumda bırakılmalı ve sürüm basmalıdır.

```go
package main

import (
	"fmt"
)

const Version = "v0.0.1"

func main() {
	// Mimari Plan 14.1 - Gün 1: main.go iskeleti
	// Henüz iş mantığı yok, yalnızca temel derlenebilirlik testi.
	fmt.Printf("OSINT Engine CLI %s\n", Version)
}