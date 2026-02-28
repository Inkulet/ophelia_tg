import { Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { map, Observable, shareReplay, throwError, catchError, of } from 'rxjs';
import { Event, Post, Project, SiteSettings, Woman, WomenPage } from '../models/cms.model';

type AnyRecord = Record<string, unknown>;

@Injectable({
  providedIn: 'root',
})
export class CmsService {
  private readonly apiBase = this.resolveApiBase();
  private settingsRequest$?: Observable<SiteSettings>;

  constructor(private readonly http: HttpClient) {}

  getSiteSettings(forceReload = false): Observable<SiteSettings> {
    if (typeof window === 'undefined') {
      return of(this.normalizeSiteSettings({}));
    }

    if (!this.settingsRequest$ || forceReload) {
      this.settingsRequest$ = this.http
        .get<AnyRecord | unknown>(`${this.apiBase}/cms/settings`)
        .pipe(
          map((item) =>
            this.normalizeSiteSettings((item as AnyRecord) ?? {}),
          ),
          catchError((err) => {
            console.error('Ошибка загрузки настроек, используем дефолтные:', err);
            return of(this.normalizeSiteSettings({}));
          }),
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

  getWomen(page: number, limit: number): Observable<WomenPage> {
    const safeLimit = Number.isFinite(limit) && limit > 0 ? Math.floor(limit) : 12;
    const safePage = Number.isFinite(page) && page > 0 ? Math.floor(page) : 1;
    const offset = (safePage - 1) * safeLimit;
    return this.http
      .get<AnyRecord | unknown>(`${this.apiBase}/api/women`, {
        params: {
          limit: String(safeLimit),
          offset: String(offset),
        },
      })
      .pipe(
        map((item) => this.normalizeWomenPage((item as AnyRecord) ?? {}, safeLimit, offset)),
      );
  }

  registerForEvent(eventId: string): Observable<{ ok: boolean }> {
    const token = this.resolveAuthToken();
    if (token === null) {
      return throwError(
        () => new Error('Не найден токен авторизации. Откройте страницу по ссылке из Telegram-бота.'),
      );
    }

    const body = { event_id: eventId };
    return this.http.post<{ ok: boolean }>(
      `${this.apiBase}/cms/events/register`,
      body,
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

  private normalizeWomenPage(item: AnyRecord, fallbackLimit: number, fallbackOffset: number): WomenPage {
    const rawItems = item['items'];
    const limit = this.pickOptionalNumber(item, ['limit', 'Limit']);
    const offset = this.pickOptionalNumber(item, ['offset', 'Offset']);
    return {
      items: Array.isArray(rawItems)
        ? rawItems.map((entry) => this.normalizeWoman((entry as AnyRecord) ?? {}))
        : [],
      limit: limit ?? fallbackLimit,
      offset: offset ?? fallbackOffset,
      total: this.pickNumber(item, ['total', 'Total']),
    };
  }

  private normalizeWoman(item: AnyRecord): Woman {
    return {
      id: this.pickNumber(item, ['id', 'ID']),
      name: this.pickString(item, ['name', 'Name']),
      biography: this.pickString(item, ['biography', 'Biography']),
      photoURL: this.pickString(item, ['photo_url', 'PhotoURL']),
      century: this.pickString(item, ['century', 'Century']),
      spheres: this.pickStringArray(item, ['spheres', 'Spheres']),
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
    const value = this.pickOptionalNumber(item, keys);
    return value ?? 0;
  }

  private pickOptionalNumber(item: AnyRecord, keys: string[]): number | null {
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
    return null;
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

  private pickStringArray(item: AnyRecord, keys: string[]): string[] {
    for (const key of keys) {
      const value = item[key];
      if (!Array.isArray(value)) {
        continue;
      }
      return value
        .map((entry) => (typeof entry === 'string' ? entry.trim() : ''))
        .filter((entry) => entry !== '');
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

  private resolveAuthToken(): string | null {
    if (typeof window === 'undefined' || typeof localStorage === 'undefined') {
      return null;
    }
    const keys = ['ophelia_cms_jwt', 'cms_jwt', 'auth_token', 'token'];
    for (const key of keys) {
      const raw = localStorage.getItem(key);
      if (!raw) {
        continue;
      }
      const token = raw.trim();
      if (token !== '') {
        return token;
      }
    }
    return null;
  }

  private resolveApiBase(): string {
    if (typeof window === 'undefined') {
      return '';
    }

    const override = window.localStorage?.getItem('ophelia_api_base')?.trim() ?? '';
    if (override !== '') {
      return override.replace(/\/+$/, '');
    }

    const { protocol, hostname, port } = window.location;
    const isLocalhost = hostname === 'localhost' || hostname === '127.0.0.1';

    if (isLocalhost && port === '4200') {
      return `${protocol}//${hostname}:8080`;
    }

    return '';
  }
}
