"""
cell.py — A simple Quilt cell.

Every file in rune-quilt is a cell with a name, scope, and contract.
This Python implementation is the byte-exact reference for the
polyformalism — Go, Rust, C, Verilog, VHDL all bind to the same hash.
"""


def new_cell(name, scope):
    """Create a fresh cell with no witnesses."""
    return {
        "name": name,
        "scope": scope,
        "contract": {},
        "value": {},
        "tier": 0,
        "witnesses": [],
    }


def bind(cell, contract):
    """BIND the cell's contract. First opcode."""
    cell["contract"] = contract
    cell["tier"] = 1
    cell["witnesses"].append({"op": "BIND", "contract": contract})
    return cell


def link(cell, other, rel="refs"):
    """LINK this cell to another. Builds the cell graph."""
    cell["value"].setdefault("links", []).append({
        "to": other["name"],
        "rel": rel,
    })
    cell["witnesses"].append({"op": "LINK", "to": other["name"], "rel": rel})
    return cell


if __name__ == "__main__":
    a = bind(new_cell("a", "demo"), {"v": 1})
    b = bind(new_cell("b", "demo"), {"v": 2})
    link(a, b, "sibling")
    print(f"cell a has {len(a['witnesses'])} witnesses")
