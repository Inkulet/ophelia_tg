import { Injectable } from '@angular/core';
import { HttpClient, HttpHeaders } from '@angular/common/http';
import { map, Observable } from 'rxjs';
import { Event, Post } from '../models/cms.model';

type ApiPost = Partial<{
  id: string;
  title: string;
  content: string;
  media_path: string;
  created_at: string;
  is_hidden: boolean;
  ID: string;
  Title: string;
  Content: string;
  MediaPath: string;
  CreatedAt: string;
  IsHidden: boolean;
}>;

type ApiEvent = Partial<{
  id: string;
  title: string;
  description: string;
  date: string;
  max_participants: number;
  current_participants: number[];
  media_path: string;
  ID: string;
  Title: string;
  Description: string;
  Date: string;
  MaxParticipants: number;
  CurrentParticipants: number[];
  MediaPath: string;
}>;

@Injectable({
  providedIn: 'root',
})
export class CmsService {
  private readonly apiBase = '';

  constructor(private readonly http: HttpClient) {}

  getPosts(): Observable<Post[]> {
    return this.http.get<ApiPost[] | unknown>(`${this.apiBase}/cms/posts`).pipe(
      map((items) =>
        Array.isArray(items)
          ? items.map((item) => this.normalizePost(item as ApiPost))
          : [],
      ),
    );
  }

  getEvents(): Observable<Event[]> {
    return this.http.get<ApiEvent[] | unknown>(`${this.apiBase}/cms/events`).pipe(
      map((items) =>
        Array.isArray(items)
          ? items.map((item) => this.normalizeEvent(item as ApiEvent))
          : [],
      ),
    );
  }

  registerForEvent(eventId: string): Observable<{ ok: boolean }> {
    const userId = this.resolveUserId();
    const body: { event_id: string; user_id?: number } = { event_id: eventId };

    let headers = new HttpHeaders();
    if (userId !== null) {
      body.user_id = userId;
      headers = headers.set('X-User-ID', String(userId));
    }

    return this.http.post<{ ok: boolean }>(
      `${this.apiBase}/cms/events/register`,
      body,
      { headers },
    );
  }

  resolveMediaUrl(path: string): string {
    const trimmed = path.trim();
    if (trimmed === '') {
      return '';
    }
    if (trimmed.startsWith('http://') || trimmed.startsWith('https://')) {
      return trimmed;
    }
    if (trimmed.startsWith('./')) {
      return `/${trimmed.slice(2)}`;
    }
    if (trimmed.startsWith('/')) {
      return trimmed;
    }
    return `/${trimmed}`;
  }

  private normalizePost(item: ApiPost): Post {
    return {
      id: item.id ?? item.ID ?? '',
      title: item.title ?? item.Title ?? '',
      content: item.content ?? item.Content ?? '',
      mediaPath: item.media_path ?? item.MediaPath ?? '',
      createdAt: item.created_at ?? item.CreatedAt ?? '',
      isHidden: item.is_hidden ?? item.IsHidden ?? false,
    };
  }

  private normalizeEvent(item: ApiEvent): Event {
    return {
      id: item.id ?? item.ID ?? '',
      title: item.title ?? item.Title ?? '',
      description: item.description ?? item.Description ?? '',
      date: item.date ?? item.Date ?? '',
      maxParticipants: item.max_participants ?? item.MaxParticipants ?? 0,
      currentParticipants:
        item.current_participants ?? item.CurrentParticipants ?? [],
      mediaPath: item.media_path ?? item.MediaPath ?? '',
    };
  }

  private resolveUserId(): number | null {
    if (typeof window === 'undefined' || typeof localStorage === 'undefined') {
      return null;
    }

    const keys = ['ophelia_user_id', 'telegram_user_id', 'user_id'];
    for (const key of keys) {
      const raw = localStorage.getItem(key);
      if (!raw) {
        continue;
      }
      const parsed = Number(raw);
      if (Number.isInteger(parsed) && parsed > 0) {
        return parsed;
      }
    }
    return null;
  }
}
