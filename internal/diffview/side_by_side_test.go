package diffview

import "testing"

func TestBuildRows_SimpleReplaceAndAdd(t *testing.T) {
    unified := `diff --git a/a.txt b/a.txt
--- a/a.txt
+++ b/a.txt
@@ -1,3 +1,4 @@
 line1
-line2
+line2 changed
 line3
+line4`

    rows := BuildRowsFromUnified(unified)
    var adds, dels, rep, ctx, hunks int
    for _, r := range rows {
        switch r.Kind {
        case RowAdd:
            adds++
        case RowDel:
            dels++
        case RowReplace:
            rep++
        case RowContext:
            ctx++
        case RowHunk:
            hunks++
        }
    }
    if hunks != 1 {
        t.Fatalf("expected 1 hunk, got %d", hunks)
    }
    if rep != 1 {
        t.Fatalf("expected 1 replace row, got %d", rep)
    }
    if adds != 1 {
        t.Fatalf("expected 1 add row, got %d", adds)
    }
    if ctx != 2 {
        t.Fatalf("expected 2 context rows, got %d", ctx)
    }
}

func TestBuildRows_DeletionOnly(t *testing.T) {
    unified := `@@ -1,2 +0,0 @@
-old1
-old2`
    rows := BuildRowsFromUnified(unified)
    var dels int
    for _, r := range rows {
        if r.Kind == RowDel {
            dels++
        }
    }
    if dels != 2 {
        t.Fatalf("expected 2 deletions, got %d", dels)
    }
}

