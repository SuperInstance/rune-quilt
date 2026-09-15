# Welcome to rune-quilt

You're inside Rune with the Quilt layer loaded. Every file you open is now a cell.
Every command you run is a witness. Every cell is canonized.

Try:
```
:quilt stats
:quilt peers "compile to rust"
:canon hash
:cell
```

Or run the standalone demo:
```
go run ./demo/ ./demo/sample-code/
```

The Quilt layer is at:
- internal/quilt/ — the cell primitive
- internal/canon/ — the canon client
- internal/a2a/   — the inter-cell client
- cmd/extension_quilt/ — the Rune extension
