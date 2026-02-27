import { Component } from '@angular/core';
import { RouterModule } from '@angular/router';
import { trigger, transition, style, animate, query, stagger } from '@angular/animations';

@Component({
  selector: 'app-home',
  standalone: true,
  imports: [RouterModule],
  template: `
    <div class="home-container">
      <!-- Левая панель - Фамилия -->
      <div class="surname-panel">
        <h1 class="surname">Торопова Ксения</h1>
      </div>

      <!-- Центральная панель - Текст с анимацией -->
      <div class="content-panel" [@listAnimation]>
        <div class="text-slide active">
          <p class="highlight-text">
            <span class="material-icons">palette</span>
            Искусствовед и культурный исследователь
          </p>

          <div class="animated-text">
            <p>Привет! Я Ксюша, студентка второго курса искусствоведения в СПГХПА имени Штиглица, работаю в музейной среде и веду авторский блог об искусстве.</p>
            <ul>

              <li><span class="material-icons">school</span>
                Мой исследовательский интерес сосредоточен на русском искусстве рубежа XIX-XX веков и искусстве Братства Прерафаэлитов.
              </li>

              <li><span class="material-icons">groups</span>
                Здесь можно познакомиться с моими выставочными проектами, записаться на экскурсию, связаться со мной для сотрудничества.
              </li>

            </ul>
          </div>

        </div>
      </div>

      <div class="photo-panel">
        <div class="photo-frame">
          <img src="/assets/photo1.jpeg" alt="Profile Photo">
        </div>
        <div class="photo-caption">
          <span class="material-icons">location_on</span>
          Санкт-Петербург | Студентка СПГХПА им. А.Л. Штиглица
        </div>
      </div>
    </div>
  `,
  animations: [
    trigger('listAnimation', [
      transition('* => *', [
        query(':enter', [
          style({ opacity: 0, transform: 'translateY(20px)' }),
          stagger(100, [
            animate('0.5s ease', style({ opacity: 1, transform: 'none' }))
          ])
        ], { optional: true })
      ])
    ])
  ]
})
export class HomeComponent {}
