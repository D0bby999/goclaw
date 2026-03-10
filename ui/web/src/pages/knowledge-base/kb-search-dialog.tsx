import { useState } from "react";
import { Search } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { useKBSearch } from "./hooks/use-kb";

interface Props {
  open: boolean;
  onClose: () => void;
  agentId: string;
}

export function KBSearchDialog({ open, onClose, agentId }: Props) {
  const [query, setQuery] = useState("");
  const { results, searching, search, setResults } = useKBSearch(agentId);

  const handleSearch = async () => {
    if (!query.trim()) return;
    await search(query.trim());
  };

  const handleClose = () => {
    setQuery("");
    setResults([]);
    onClose();
  };

  return (
    <Dialog open={open} onOpenChange={(v) => !v && handleClose()}>
      <DialogContent className="max-w-2xl max-h-[80vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Search Knowledge Base</DialogTitle>
        </DialogHeader>
        <div className="flex gap-2">
          <Input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Search across all collections..."
            onKeyDown={(e) => e.key === "Enter" && handleSearch()}
          />
          <Button onClick={handleSearch} disabled={searching || !query.trim()}>
            <Search className="h-4 w-4 mr-1" />
            {searching ? "..." : "Search"}
          </Button>
        </div>

        {results.length > 0 && (
          <div className="space-y-3 mt-4">
            <p className="text-sm text-muted-foreground">{results.length} result{results.length !== 1 ? "s" : ""}</p>
            {results.map((r, i) => (
              <div key={i} className="border rounded-lg p-3 space-y-1">
                <div className="flex items-center gap-2">
                  <span className="font-medium text-sm">{r.filename}</span>
                  <Badge variant="outline" className="text-xs">
                    score: {r.score.toFixed(2)}
                  </Badge>
                  <span className="text-xs text-muted-foreground">
                    chunk #{r.chunk_index} (lines {r.start_line}-{r.end_line})
                  </span>
                </div>
                <pre className="text-xs text-muted-foreground whitespace-pre-wrap bg-muted/50 rounded p-2 max-h-32 overflow-y-auto">
                  {r.text}
                </pre>
              </div>
            ))}
          </div>
        )}

        {!searching && results.length === 0 && query && (
          <p className="text-sm text-muted-foreground text-center py-4">
            No results. Try a different query.
          </p>
        )}
      </DialogContent>
    </Dialog>
  );
}
