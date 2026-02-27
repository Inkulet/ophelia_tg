import { Injectable } from '@angular/core';
import { HttpClient, HttpHeaders } from '@angular/common/http';
import { catchError, map, Observable, of, shareReplay, throwError } from 'rxjs';
import { Event, NewsPost, Post, Project, SiteSettings } from '../models/cms.model';

type AnyRecord = Record<string, unknown>;

@Injectable({
  providedIn: 'root',
})
export class CmsService {
  private readonly apiBase = '';
  private settingsRequest$?: Observable<SiteSettings>;

  constructor(private readonly http: HttpClient) {}

  getSiteSettings(forceReload = false): Observable<SiteSettings> {
    if (!this.settingsRequest$ || forceReload) {
      this.settingsRequest$ = this.http
        .get<AnyRecord | unknown>(`${this.apiBase}/cms/settings`)
        .pipe(
          map((item) =>
            this.normalizeSiteSettings((item as AnyRecord) ?? {}),
          ),
          catchError(() => of(this.emptySiteSettings())),
          shareReplay(1),
        );
    }
    return this.settingsRequest$;
  }

  getProjects(): Observable<Project[]> {
    return this.http
      .get<AnyRecord[] | unknown>(`${this.apiBase}/cms/projects`)
      .pipe(
        map((items) =>
          Array.isArray(items)
            ? items.map((item) => this.normalizeProject(item as AnyRecord))
            : [],
        ),
      );
  }

  getPosts(): Observable<Post[]> {
    return this.http
      .get<AnyRecord[] | unknown>(`${this.apiBase}/cms/posts`)
      .pipe(
        map((items) =>
          Array.isArray(items)
            ? items.map((item) => this.normalizePost(item as AnyRecord))
            : [],
        ),
      );
  }

  getEvents(): Observable<Event[]> {
    return this.http
      .get<AnyRecord[] | unknown>(`${this.apiBase}/cms/events`)
      .pipe(
        map((items) =>
          Array.isArray(items)
            ? items.map((item) => this.normalizeEvent(item as AnyRecord))
            : [],
        ),
      );
  }

  getNews(): Observable<NewsPost[]> {
    return this.http
      .get<AnyRecord[] | unknown>(`${this.apiBase}/cms/news`)
      .pipe(
        map((items) =>
          Array.isArray(items)
            ? items.map((item) => this.normalizeNews(item as AnyRecord))
            : [],
        ),
      );
  }

  registerForEvent(eventId: string): Observable<{ ok: boolean }> {
    const userId = this.resolveUserId();
    if (userId === null) {
      return throwError(
        () => new Error('Не найден user_id. Откройте страницу из Telegram-бота.'),
      );
    }

    const headers = new HttpHeaders().set('X-User-ID', String(userId));
    const body = { event_id: eventId, user_id: userId };
    return this.http.post<{ ok: boolean }>(
      `${this.apiBase}/cms/events/register`,
      body,
      { headers },
    );
  }

  resolveMediaUrl(path: string): string {
    const trimmed = (path ?? '').trim();
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

  private normalizeSiteSettings(item: AnyRecord): SiteSettings {
    return {
      id: this.pickString(item, ['id', 'ID']),
      backgroundURL: this.pickString(item, ['background_url', 'BackgroundURL']),
      avatarURL: this.pickString(item, ['avatar_url', 'AvatarURL']),
      homeDescription: this.pickString(item, [
        'home_description',
        'HomeDescription',
      ]),
      aboutText: this.pickString(item, ['about_text', 'AboutText']),
      contactEmail: this.pickString(item, ['contact_email', 'ContactEmail']),
      contactPhone: this.pickString(item, ['contact_phone', 'ContactPhone']),
      contactLocation: this.pickString(item, [
        'contact_location',
        'ContactLocation',
      ]),
    };
  }

  private emptySiteSettings(): SiteSettings {
    return {
      id: '',
      backgroundURL: '',
      avatarURL: '',
      homeDescription: '',
      aboutText: '',
      contactEmail: '',
      contactPhone: '',
      contactLocation: '',
    };
  }

  private normalizeProject(item: AnyRecord): Project {
    return {
      id: this.pickString(item, ['id', 'ID']),
      title: this.pickString(item, ['title', 'Title']),
      shortDescription: this.pickString(item, [
        'short_description',
        'ShortDescription',
      ]),
      detailedContent: this.pickString(item, [
        'detailed_content',
        'DetailedContent',
      ]),
      mediaURL: this.pickString(item, ['media_url', 'MediaURL']),
    };
  }

  private normalizeEvent(item: AnyRecord): Event {
    return {
      id: this.pickString(item, ['id', 'ID']),
      title: this.pickString(item, ['title', 'Title']),
      description: this.pickString(item, ['description', 'Description']),
      date: this.pickString(item, ['date', 'Date']),
      time: this.pickString(item, ['time', 'Time']),
      location: this.pickString(item, ['location', 'Location']),
      maxParticipants: this.pickNumber(item, [
        'max_participants',
        'MaxParticipants',
      ]),
      currentParticipants: this.pickNumberArray(item, [
        'current_participants',
        'CurrentParticipants',
      ]),
    };
  }

  private normalizeNews(item: AnyRecord): NewsPost {
    return {
      id: this.pickString(item, ['id', 'ID']),
      text: this.pickString(item, ['text', 'Text']),
      imageURL: this.pickString(item, ['image_url', 'ImageURL']),
      postURL: this.pickString(item, ['post_url', 'PostURL']),
    };
  }

  private normalizePost(item: AnyRecord): Post {
    return {
      id: this.pickString(item, ['id', 'ID']),
      title: this.pickString(item, ['title', 'Title']),
      content: this.pickString(item, ['content', 'Content']),
      mediaPath: this.pickString(item, ['media_path', 'MediaPath']),
      createdAt: this.pickString(item, ['created_at', 'CreatedAt']),
      isHidden: this.pickBool(item, ['is_hidden', 'IsHidden']),
    };
  }

  private pickString(item: AnyRecord, keys: string[]): string {
    for (const key of keys) {
      const value = item[key];
      if (typeof value === 'string') {
        return value;
      }
    }
    return '';
  }

  private pickNumber(item: AnyRecord, keys: string[]): number {
    for (const key of keys) {
      const value = item[key];
      if (typeof value === 'number' && Number.isFinite(value)) {
        return value;
      }
      if (typeof value === 'string') {
        const parsed = Number(value);
        if (Number.isFinite(parsed)) {
          return parsed;
        }
      }
    }
    return 0;
  }

  private pickNumberArray(item: AnyRecord, keys: string[]): number[] {
    for (const key of keys) {
      const value = item[key];
      if (!Array.isArray(value)) {
        continue;
      }
      const parsed = value
        .map((entry) => Number(entry))
        .filter((entry) => Number.isFinite(entry));
      return parsed;
    }
    return [];
  }

  private pickBool(item: AnyRecord, keys: string[]): boolean {
    for (const key of keys) {
      const value = item[key];
      if (typeof value === 'boolean') {
        return value;
      }
      if (typeof value === 'number') {
        return value !== 0;
      }
      if (typeof value === 'string') {
        const low = value.trim().toLowerCase();
        if (low === 'true' || low === '1') {
          return true;
        }
        if (low === 'false' || low === '0') {
          return false;
        }
      }
    }
    return false;
  }

  private resolveUserId(): number | null {
    if (typeof window === 'undefined' || typeof localStorage === 'undefined') {
      return null;
    }
    const keys = ['ophelia_user_id', 'telegram_user_id', 'tg_user_id', 'user_id'];
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
