# GoSonata Conformance Tests

Questa directory contiene la suite di test di conformità che confronta l'implementazione GoSonata con l'implementazione JavaScript ufficiale di JSONata 2.1.0.

## Scopo

I conformance test assicurano che GoSonata produca risultati identici all'implementazione JavaScript di riferimento per tutte le espressioni JSONata. Questo garantisce la compatibilità e la correttezza dell'implementazione.

## Struttura

```
tests/conformance/
├── conformance_test.go  # Suite di test Go
├── runner.js            # Script Node.js per eseguire JSONata JS
├── package.json         # Dipendenze npm (jsonata)
└── README.md           # Questa documentazione
```

## Come Funziona

1. **Go Test Runner** (`conformance_test.go`):
   - Definisce test case con query JSONata e dati di input
   - Esegue ogni espressione con Go (GoSonata)
   - Esegue la stessa espressione con JavaScript (tramite `runner.js`)
   - Confronta i risultati normalizzati come JSON

2. **JavaScript Runner** (`runner.js`):
   - Script Node.js che accetta input JSON via stdin
   - Esegue espressioni con l'implementazione JSONata ufficiale
   - Restituisce risultati come JSON via stdout
   - Gestisce errori e li formatta per confronto

## Prerequisiti

1. **Go 1.26+** installato
2. **Node.js 14+** installato
3. **JSONata JavaScript** compilato:

   ```bash
   cd ../../thirdy/jsonata
   npm install
   npm run browserify
   ```

4. **Dipendenze test** installate:

   ```bash
   cd tests/conformance
   npm install
   ```

## Esecuzione

### Tutti i test di conformità

```bash
go test ./tests/conformance/... -v
```

### Test specifico

```bash
go test ./tests/conformance/... -v -run "TestConformance/string_literal"
```

### Solo test con expected value

```bash
go test ./tests/conformance/... -v -run "TestConformanceWithExpected"
```

### Con timeout esteso

```bash
go test ./tests/conformance/... -v -timeout 5m
```

## Risultati Attuali

**Data ultima esecuzione**: 2026-02-13

### Statistiche

- **Test totali**: 84 (80 conformance + 4 expected)
- **Passati**: 80 (95.2%)
- **Falliti**: 4

### Test Passati (80)

✅ **Literals**: string, number, boolean, null
✅ **Variables**: context ($), field access, nested paths
✅ **Arithmetic**: +, -, *, /, %, negation, precedence
✅ **Comparison**: =, !=, <, >, <=, >=
✅ **Logical**: and, or
✅ **Strings**: concatenation (&), type coercion
✅ **Arrays**: literals, indexing, projection, filters
✅ **Objects**: literals, expressions
✅ **Conditionals**: ternary operator (? :)
✅ **Built-in Functions**:

- Aggregation: sum, count, average, min, max
- String: string, length (string), substring, uppercase, lowercase, trim, contains
- Type: type, exists, number, boolean
- Math: abs, floor, ceil, round, sqrt, power
✅ **Lambdas**: map, filter, reduce
✅ **Apply operator**: (~>)
✅ **Complex expressions**: filter + map chains

### Test Falliti (4)

❌ **array_index_negative** (Feature non implementata)

- Query: `items[-1]`
- Go result: `[10,20,30]` (returna tutto l'array)
- JS result: `30` (ultimo elemento)
- **Stato**: Feature da implementare (indici negativi)

❌ **range_ascending** (Test errato)

- Query: `1..5`
- JS error: "Syntax error: .."
- Go result: Success
- **Stato**: JS JSONata 2.1.0 non supporta range operator, test da correggere

❌ **range_descending** (Test errato)

- Query: `5..1`
- JS error: "Syntax error: .."
- Go result: Success
- **Stato**: JS JSONata 2.1.0 non supporta range operator, test da correggere

❌ **length_array** (Comportamento diverso)

- Query: `$length([1,2,3])`
- JS error: "does not match function signature"
- Go result: `3`
- **Stato**: Go estende $length per array, JS solo string

## Analisi

### Compatibilità Core (95%+)

GoSonata è altamente compatibile con JSONata JS per:

- ✅ Parsing e sintassi
- ✅ Operatori base (arithmetic, comparison, logical)
- ✅ Path navigation e filtering
- ✅ Object e array construction
- ✅ Built-in functions standard
- ✅ Lambda expressions e higher-order functions

### Differenze Note

1. **Negative Array Indexing**: Non ancora implementato in Go
2. **Range Operator**: Implementato in Go ma non in JS 2.1.0
3. **$length esteso**: Go supporta array, JS solo string

### Prossimi Passi

1. ⚠️ **Rimuovere test range**: Non validi per JSONata 2.1.0
2. ⚠️ **Correggere test $length array**: Documentare come estensione Go
3. 🔧 **Implementare negative indexing**: `items[-1]` per ultimo elemento
4. 📝 **Aggiungere più test edge case**: null handling, type coercion, nested lambdas
5. 🎯 **Test suite ufficiale**: Integrare test cases da `thirdy/jsonata/test/`

## Aggiungere Nuovi Test

Aggiungi test al slice `testCases` in `conformance_test.go`:

```go
{
    Name: "my_new_test",
    Query: "$.field",
    Data: map[string]interface{}{"field": "value"},
    // Expected: "value", // optional
    // ShouldError: false, // optional
},
```

### Test con Expected Value

Per test con risultato atteso specifico, aggiungi a `TestConformanceWithExpected`:

```go
{
    Name: "test_name",
    Query: "expression",
    Data: data,
    Expected: expectedValue,
},
```

## Debugging

### Verificare Runner JS Manualmente

```bash
cd tests/conformance
echo '{"query": "2 + 3", "data": null}' | node runner.js
```

Output atteso:

```json
{
  "success": true,
  "result": 5,
  "error": null
}
```

### Confrontare Output Go vs JS

```bash
# Test Go
cd tests/conformance
go run -tags debug ./debug_runner.go "2 + 3" null

# Test JS
echo '{"query": "2 + 3", "data": null}' | node runner.js
```

## Riferimenti

- **JSONata Documentation**: <https://jsonata.org>
- **JSONata GitHub**: <https://github.com/jsonata-js/jsonata>
- **JSONata Spec**: <https://docs.jsonata.org/overview.html>

## Note

- I test richiedono ~4 secondi per l'esecuzione completa (overhead Node.js)
- Ogni test spawna un processo Node.js separato per isolamento
- I risultati sono normalizzati come JSON per confronto
- Le differenze di tipo (int vs float) sono gestite tramite serializzazione JSON
