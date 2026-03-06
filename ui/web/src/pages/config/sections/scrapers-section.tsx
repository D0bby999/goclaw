import { useState } from "react";
import { Trash2, Plus, LogIn, Loader2, Cookie } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { Textarea } from "@/components/ui/textarea";
import { Badge } from "@/components/ui/badge";
import { useScraperCookies } from "../hooks/use-scraper-cookies";

const PLATFORMS = [
  { value: "facebook", label: "Facebook" },
  { value: "instagram", label: "Instagram" },
] as const;

export function ScrapersSection() {
  const { cookies, loading, loginPending, addManual, remove, startLogin } = useScraperCookies();
  const [showManual, setShowManual] = useState(false);
  const [manualPlatform, setManualPlatform] = useState("facebook");
  const [manualLabel, setManualLabel] = useState("");
  const [manualCookies, setManualCookies] = useState("");
  const [saving, setSaving] = useState(false);

  const handleManualSave = async () => {
    if (!manualPlatform || !manualLabel || !manualCookies) return;
    setSaving(true);
    try {
      await addManual(manualPlatform, manualLabel, manualCookies);
      setShowManual(false);
      setManualLabel("");
      setManualCookies("");
    } finally {
      setSaving(false);
    }
  };

  const handleLogin = async (platform: string) => {
    try {
      await startLogin(platform);
    } catch {
      // error handled by ws layer
    }
  };

  return (
    <>
      <Card>
        <CardHeader className="pb-3">
          <div className="flex items-center justify-between">
            <div>
              <CardTitle className="text-base">Scraper Cookies</CardTitle>
              <CardDescription>
                Manage login cookies for Facebook &amp; Instagram scrapers.
                Cookies are encrypted at rest.
              </CardDescription>
            </div>
            <div className="flex items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                className="gap-1.5"
                onClick={() => handleLogin("facebook")}
                disabled={!!loginPending}
              >
                {loginPending === "facebook" ? (
                  <Loader2 className="h-3.5 w-3.5 animate-spin" />
                ) : (
                  <LogIn className="h-3.5 w-3.5" />
                )}
                Login Facebook
              </Button>
              <Button
                variant="outline"
                size="sm"
                className="gap-1.5"
                onClick={() => handleLogin("instagram")}
                disabled={!!loginPending}
              >
                {loginPending === "instagram" ? (
                  <Loader2 className="h-3.5 w-3.5 animate-spin" />
                ) : (
                  <LogIn className="h-3.5 w-3.5" />
                )}
                Login Instagram
              </Button>
              <Button
                variant="outline"
                size="sm"
                className="gap-1.5"
                onClick={() => setShowManual(true)}
              >
                <Plus className="h-3.5 w-3.5" /> Paste
              </Button>
            </div>
          </div>
        </CardHeader>
        <CardContent>
          {loginPending && (
            <div className="mb-3 flex items-center gap-2 rounded-md border border-blue-500/30 bg-blue-500/5 px-3 py-2 text-sm text-blue-700 dark:text-blue-400">
              <Loader2 className="h-4 w-4 animate-spin" />
              Waiting for login... (browser opened for {loginPending})
            </div>
          )}

          {loading ? (
            <p className="text-sm text-muted-foreground">Loading...</p>
          ) : cookies.length === 0 ? (
            <div className="flex flex-col items-center gap-2 py-6 text-center">
              <Cookie className="h-8 w-8 text-muted-foreground/50" />
              <p className="text-sm text-muted-foreground">
                No scraper cookies configured. Use the login buttons above or paste cookies manually.
              </p>
            </div>
          ) : (
            <div className="space-y-2">
              {cookies.map((entry) => (
                <div
                  key={`${entry.platform}-${entry.label}`}
                  className="flex items-center justify-between gap-3 rounded-md border px-3 py-2"
                >
                  <div className="flex items-center gap-3 min-w-0">
                    <Badge variant="secondary" className="shrink-0 capitalize">
                      {entry.platform}
                    </Badge>
                    <span className="text-sm font-medium truncate">{entry.label}</span>
                    <span className="text-xs text-muted-foreground font-mono truncate">
                      {entry.cookies}
                    </span>
                  </div>
                  <div className="flex items-center gap-2 shrink-0">
                    {entry.updated_at && (
                      <span className="text-xs text-muted-foreground">
                        {new Date(entry.updated_at).toLocaleDateString()}
                      </span>
                    )}
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-7 w-7"
                      onClick={() => remove(entry.platform, entry.label)}
                    >
                      <Trash2 className="h-3.5 w-3.5 text-muted-foreground" />
                    </Button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      <Dialog open={showManual} onOpenChange={setShowManual}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Add Cookie Manually</DialogTitle>
          </DialogHeader>
          <div className="space-y-4">
            <div className="grid gap-1.5">
              <Label>Platform</Label>
              <Select value={manualPlatform} onValueChange={setManualPlatform}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {PLATFORMS.map((p) => (
                    <SelectItem key={p.value} value={p.value}>
                      {p.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="grid gap-1.5">
              <Label>Label</Label>
              <Input
                value={manualLabel}
                onChange={(e) => setManualLabel(e.target.value)}
                placeholder="e.g. my-account"
              />
            </div>
            <div className="grid gap-1.5">
              <Label>Cookies</Label>
              <Textarea
                value={manualCookies}
                onChange={(e) => setManualCookies(e.target.value)}
                placeholder="c_user=123; xs=abc (Facebook) or sessionid=xyz (Instagram)"
                rows={3}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setShowManual(false)}>
              Cancel
            </Button>
            <Button
              onClick={handleManualSave}
              disabled={saving || !manualLabel || !manualCookies}
            >
              {saving ? "Saving..." : "Save"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
