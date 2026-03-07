import { useState } from "react";
import { Plus, Trash2, Image, Film, FileAudio } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

export interface MediaItem {
  url: string;
  media_type: string;
  filename?: string;
}

interface MediaAttachmentsProps {
  items: MediaItem[];
  onChange: (items: MediaItem[]) => void;
}

const MEDIA_TYPES = [
  { value: "image", label: "Image", icon: Image },
  { value: "video", label: "Video", icon: Film },
  { value: "gif", label: "GIF", icon: Image },
  { value: "audio", label: "Audio", icon: FileAudio },
];

export function MediaAttachments({ items, onChange }: MediaAttachmentsProps) {
  const [url, setUrl] = useState("");
  const [mediaType, setMediaType] = useState("image");

  const addMedia = () => {
    if (!url.trim()) return;
    onChange([...items, { url: url.trim(), media_type: mediaType }]);
    setUrl("");
  };

  const removeMedia = (index: number) => {
    onChange(items.filter((_, i) => i !== index));
  };

  return (
    <div className="space-y-3">
      <label className="text-sm font-medium">Media</label>

      {items.length > 0 && (
        <div className="space-y-2">
          {items.map((item, i) => {
            const typeMeta = MEDIA_TYPES.find((t) => t.value === item.media_type);
            const Icon = typeMeta?.icon ?? Image;
            return (
              <div key={i} className="flex items-center gap-2 rounded-md border px-3 py-2 text-sm">
                <Icon className="h-4 w-4 shrink-0 text-muted-foreground" />
                <span className="flex-1 truncate">{item.url}</span>
                <span className="text-xs text-muted-foreground">{item.media_type}</span>
                <Button variant="ghost" size="icon" className="h-6 w-6" onClick={() => removeMedia(i)}>
                  <Trash2 className="h-3 w-3" />
                </Button>
              </div>
            );
          })}
        </div>
      )}

      <div className="flex gap-2">
        <Input
          value={url}
          onChange={(e) => setUrl(e.target.value)}
          placeholder="Media URL..."
          className="flex-1"
          onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); addMedia(); } }}
        />
        <Select value={mediaType} onValueChange={setMediaType}>
          <SelectTrigger className="w-24">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {MEDIA_TYPES.map((t) => (
              <SelectItem key={t.value} value={t.value}>{t.label}</SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Button variant="outline" size="icon" onClick={addMedia} disabled={!url.trim()}>
          <Plus className="h-4 w-4" />
        </Button>
      </div>
    </div>
  );
}
