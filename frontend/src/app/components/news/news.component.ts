import { CommonModule } from '@angular/common';
import { Component, OnInit } from '@angular/core';
import { NewsPost } from '../../models/cms.model';
import { CmsService } from '../../services/cms.service';

@Component({
  selector: 'app-news',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './news.component.html',
  styleUrls: ['./news.component.css'],
})
export class NewsComponent implements OnInit {
  posts: NewsPost[] = [];
  loading = true;
  error = '';

  constructor(private readonly cmsService: CmsService) {}

  ngOnInit(): void {
    this.cmsService.getNews().subscribe({
      next: (posts) => {
        this.posts = posts;
        this.loading = false;
      },
      error: () => {
        this.error = 'Не удалось загрузить новости канала.';
        this.loading = false;
      },
    });
  }
}
