import { Component, ViewChild, TemplateRef, ElementRef, Renderer2, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';

interface TourImage {
  src: string;
  alt: string;
}

interface Tour {
  id: number;
  title: string;
  subtitle: string;
  badge: string;
  description: string;
  images: TourImage[];
  location: string;
  schedule: string;
  group: string;
  museumPhoto: string;
  mapImage: string;
  mapLink: string;
  metro: string;
  quote?: string;
  liked?: boolean; // Новое поле для отслеживания лайков
}

@Component({
  selector: 'app-skills',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './skills.component.html',
  styleUrls: ['./skills.component.css']
})
export class SkillsComponent implements OnInit {
  @ViewChild('expandedCardTemplate') expandedCardTemplate!: TemplateRef<any>;

  selectedTour: Tour | null = null;
  expandedCardElement: HTMLElement | null = null;

  // Массив для хранения отсортированных экскурсий
  sortedTours: Tour[] = [];

  // Исходные данные экскурсий
  tours: Tour[] = [
    {
      id: 0,
      title: 'Анималистика. И в шутку, и всерьез.',
      subtitle: 'Авторская экскурсия (еженедельно)',
      badge: 'Еженедельно',
      description: 'Интерактивная экскурсия через призму современного искусства, где зрители становятся участниками художественного диалога. Маршрут построен на контрасте классической музейной архитектуры и экспериментальных арт-объектов.',
      images: [
        { src: '/assets/excursion1.jpeg', alt: 'Экспонаты выставки' },
        { src: '/assets/excursion2.jpeg', alt: 'Рабочий процесс' },
        { src: '/assets/excursion3.jpeg', alt: 'Интерактив с посетителями' }
      ],
      location: 'Музей искусства Санкт-Петербурга XX–XXI веков (МИСП), наб. канала Грибоедова, 103',
      schedule: 'Каждую субботу в 15:00',
      group: 'до 15 человек',
      museumPhoto: '/assets/misp-building.jpg',
      mapImage: 'https://static-maps.yandex.ru/1.x/?ll=30.300037,59.926478&z=16&size=600,300&l=map&pt=30.300037,59.926478,pm2rdl',
      mapLink: 'https://yandex.ru/maps/2/saint-petersburg/?ll=30.300037%2C59.926478&z=16&mode=search&text=МИСП',
      metro: 'Ближайшее метро: Садовая (10 мин.)',
      quote: 'Искусство живет в диалоге - между пространством и временем, художником и зрителем, прошлым и будущим',
      liked: false
    },
    {
      id: 1,
      title: 'Бла бла бла',
      subtitle: 'Авторская экскурсия (еженедельно)',
      badge: 'Еженедельно',
      description: 'Интерактивная экскурсия через призму современного искусства, где зрители становятся участниками художественного диалога. Маршрут построен на контрасте классической музейной архитектуры и экспериментальных арт-объектов.',
      images: [
        { src: '/assets/excursion1.jpeg', alt: 'Экспонаты выставки' },
        { src: '/assets/excursion2.jpeg', alt: 'Рабочий процесс' },
        { src: '/assets/excursion3.jpeg', alt: 'Интерактив с посетителями' }
      ],
      location: 'Музей искусства Санкт-Петербурга XX–XXI веков (МИСП), наб. канала Грибоедова, 103',
      schedule: 'Каждую субботу в 15:00',
      group: 'до 15 человек',
      museumPhoto: '/assets/misp-building.jpg',
      mapImage: 'https://static-maps.yandex.ru/1.x/?ll=30.300037,59.926478&z=16&size=600,300&l=map&pt=30.300037,59.926478,pm2rdl',
      mapLink: 'https://yandex.ru/maps/2/saint-petersburg/?ll=30.300037%2C59.926478&z=16&mode=search&text=МИСП',
      metro: 'Ближайшее метро: Садовая (10 мин.)',
      quote: 'Искусство живет в диалоге - между пространством и временем, художником и зрителем, прошлым и будущим',
      liked: false
    }
  ];

  constructor(private renderer: Renderer2, private el: ElementRef) {}

  ngOnInit() {
    // Загрузка лайков из localStorage
    this.loadLikedTours();

    // Сортировка экскурсий (лайкнутые сверху)
    this.sortTours();

    // Инициализация обработчика прокрутки для индикаторов
    setTimeout(() => {
      this.initScrollIndicators();
    }, 500);
  }

  // Загрузка лайкнутых экскурсий из localStorage
  loadLikedTours() {
    try {
      const preferences = localStorage.getItem('laglaneuse_user_preferences');
      if (preferences) {
        const data = JSON.parse(preferences);
        if (data.likedItems && data.likedItems.skills) {
          // Обновляем состояние лайков в массиве экскурсий
          this.tours.forEach(tour => {
            tour.liked = data.likedItems.skills.includes(tour.id);
          });
        }
      }
    } catch (error) {
      console.error('Ошибка при загрузке лайков из localStorage:', error);
    }
  }

  // Сохранение лайкнутых экскурсий в localStorage
  saveLikedTours() {
    try {
      // Получаем текущие предпочтения пользователя
      let preferences = {};
      const savedPreferences = localStorage.getItem('laglaneuse_user_preferences');
      if (savedPreferences) {
        preferences = JSON.parse(savedPreferences);
      }

      // Собираем ID лайкнутых экскурсий
      const likedTourIds = this.tours.filter(tour => tour.liked).map(tour => tour.id);

      // Обновляем данные
      preferences = {
        ...preferences,
        likedItems: {
          ...(preferences as any).likedItems,
          skills: likedTourIds
        },
        lastVisit: new Date().toISOString()
      };

      // Сохраняем в localStorage
      localStorage.setItem('laglaneuse_user_preferences', JSON.stringify(preferences));
    } catch (error) {
      console.error('Ошибка при сохранении лайков в localStorage:', error);
    }
  }

  // Сортировка экскурсий (лайкнутые сверху)
  sortTours() {
    // Создаем копию массива для сортировки
    this.sortedTours = [...this.tours];

    // Сортируем: сначала лайкнутые, затем остальные в исходном порядке
    this.sortedTours.sort((a, b) => {
      if (a.liked && !b.liked) return -1;
      if (!a.liked && b.liked) return 1;
      return 0;
    });
  }

  // Обработка клика по кнопке лайка
  toggleLike(event: Event, tourId: number) {
    event.stopPropagation(); // Предотвращаем открытие карточки

    // Находим экскурсию по ID
    const tour = this.tours.find(t => t.id === tourId);
    if (tour) {
      // Инвертируем состояние лайка
      tour.liked = !tour.liked;

      // Сохраняем изменения в localStorage
      this.saveLikedTours();

      // Пересортировываем экскурсии
      this.sortTours();
    }
  }

  initScrollIndicators() {
    const cardsWrapper = this.el.nativeElement.querySelector('.cards-wrapper');
    const dots = this.el.nativeElement.querySelectorAll('.scroll-indicator .dot');

    if (cardsWrapper && dots.length) {
      cardsWrapper.addEventListener('scroll', () => {
        const scrollPosition = cardsWrapper.scrollLeft;
        const cardWidth = cardsWrapper.querySelector('.tour-card').offsetWidth + 24; // Ширина карточки + отступ
        const activeIndex = Math.round(scrollPosition / cardWidth);

        dots.forEach((dot: Element, index: number) => {
          if (index === activeIndex) {
            this.renderer.addClass(dot, 'active');
          } else {
            this.renderer.removeClass(dot, 'active');
          }
        });
      });
    }
  }

  expandCard(event: Event, tour: Tour) {
    event.stopPropagation();

    // Устанавливаем выбранную экскурсию
    this.selectedTour = tour;

    // Создаем элемент из шаблона
    const viewContainerRef = this.el.nativeElement;
    const embeddedViewRef = this.expandedCardTemplate.createEmbeddedView({ $implicit: this.selectedTour });

    // Добавляем элемент в DOM
    embeddedViewRef.detectChanges();
    const rootNodes = embeddedViewRef.rootNodes;

    if (rootNodes.length > 0) {
      this.expandedCardElement = rootNodes[0];
      this.renderer.appendChild(document.body, this.expandedCardElement);

      // Блокируем прокрутку основной страницы
      this.renderer.addClass(document.body, 'no-scroll');

      // Инициализируем галерею
      setTimeout(() => {
        this.initGallerySlider();
      }, 100);
    }
  }

  closeExpandedCard() {
    if (this.expandedCardElement) {
      // Удаляем элемент из DOM
      this.renderer.removeChild(document.body, this.expandedCardElement);
      this.expandedCardElement = null;

      // Разблокируем прокрутку основной страницы
      this.renderer.removeClass(document.body, 'no-scroll');
    }
  }

  initGallerySlider() {
    if (!this.expandedCardElement) return;

    const slider = this.expandedCardElement.querySelector('.gallery-slider');
    const dots = this.expandedCardElement.querySelectorAll('.gallery-dots .dot');

    if (slider && dots.length) {
      slider.addEventListener('scroll', () => {
        const scrollPosition = (slider as HTMLElement).scrollLeft;
        const imageWidth = (slider.querySelector('.gallery-image') as HTMLElement).offsetWidth;
        const activeIndex = Math.round(scrollPosition / imageWidth);

        dots.forEach((dot: Element, index: number) => {
          if (index === activeIndex) {
            this.renderer.addClass(dot, 'active');
          } else {
            this.renderer.removeClass(dot, 'active');
          }
        });
      });

      // Добавляем обработчики для точек навигации
      dots.forEach((dot: Element, index: number) => {
        dot.addEventListener('click', () => {
          const imageWidth = (slider.querySelector('.gallery-image') as HTMLElement).offsetWidth;
          (slider as HTMLElement).scrollTo({
            left: index * imageWidth,
            behavior: 'smooth'
          });
        });
      });
    }
  }

  // Обработка лайка в развернутой карточке
  toggleLikeInExpandedCard(event: Event) {
    if (!this.selectedTour) return;

    event.stopPropagation();

    // Инвертируем состояние лайка
    this.selectedTour.liked = !this.selectedTour.liked;

    // Обновляем состояние в основном массиве
    const tour = this.tours.find(t => t.id === this.selectedTour?.id);
    if (tour) {
      tour.liked = this.selectedTour.liked;
    }

    // Сохраняем изменения в localStorage
    this.saveLikedTours();

    // Пересортировываем экскурсии
    this.sortTours();
  }
}
