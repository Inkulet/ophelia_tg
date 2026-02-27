import { CommonModule, isPlatformBrowser } from '@angular/common';
import { HttpErrorResponse } from '@angular/common/http';
import { Component, Inject, OnInit, PLATFORM_ID } from '@angular/core';
import { Event } from '../../models/cms.model';
import { CmsService } from '../../services/cms.service';

@Component({
  selector: 'app-event-list',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './event-list.component.html',
  styleUrls: ['./event-list.component.css'],
})
export class EventListComponent implements OnInit {
  events: Event[] = [];
  loading = true;
  error = '';
  actionState: Record<string, { success?: string; error?: string; loading: boolean }> =
    {};

  constructor(
    private readonly cmsService: CmsService,
    @Inject(PLATFORM_ID) private readonly platformId: object,
  ) {}

  ngOnInit(): void {
    if (!isPlatformBrowser(this.platformId)) {
      this.loading = false;
      return;
    }
    this.loadEvents();
  }

  register(event: Event): void {
    if (this.isFull(event)) {
      return;
    }

    this.actionState[event.id] = { loading: true };
    this.cmsService.registerForEvent(event.id).subscribe({
      next: () => {
        const updated = this.events.map((item) => {
          if (item.id !== event.id) {
            return item;
          }
          return {
            ...item,
            currentParticipants: [...item.currentParticipants, -1],
          };
        });
        this.events = updated;
        this.actionState[event.id] = {
          loading: false,
          success: 'Вы успешно записаны.',
        };
      },
      error: (err: unknown) => {
        this.actionState[event.id] = {
          loading: false,
          error: this.extractError(err),
        };
      },
    });
  }

  participantsLabel(event: Event): string {
    const current = event.currentParticipants.length;
    if (event.maxParticipants <= 0) {
      return `${current} участника(ов)`;
    }
    return `${current} / ${event.maxParticipants}`;
  }

  isFull(event: Event): boolean {
    return (
      event.maxParticipants > 0 &&
      event.currentParticipants.length >= event.maxParticipants
    );
  }

  mediaUrl(path: string): string {
    return this.cmsService.resolveMediaUrl(path);
  }

  isVideo(path: string): boolean {
    return path.toLowerCase().endsWith('.mp4');
  }

  private loadEvents(): void {
    this.cmsService.getEvents().subscribe({
      next: (events) => {
        this.events = events;
        this.loading = false;
      },
      error: () => {
        this.error = 'Не удалось загрузить мероприятия.';
        this.loading = false;
      },
    });
  }

  private extractError(err: unknown): string {
    if (err instanceof Error && err.message.trim() !== '') {
      return err.message;
    }
    if (err instanceof HttpErrorResponse) {
      if (typeof err.error === 'object' && err.error && 'error' in err.error) {
        const value = err.error.error;
        if (typeof value === 'string' && value.trim() !== '') {
          return value;
        }
      }
      if (err.status === 409) {
        return 'Места закончились.';
      }
    }
    return 'Не удалось записаться.';
  }
}
