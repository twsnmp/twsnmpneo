export type ModalType = 'danger' | 'warning' | 'info';

export interface ConfirmOptions {
  title?: string;
  message: string;
  detail?: string;
  confirmText?: string;
  cancelText?: string;
  type?: ModalType;
}

export interface LoadingOptions {
  title?: string;
  message?: string;
  cancelable?: boolean;
  onCancel?: () => void;
}

export interface AlertOptions {
  title?: string;
  message: string;
  detail?: string;
  confirmText?: string;
  type?: ModalType;
}

interface ConfirmState {
  show: boolean;
  title: string;
  message: string;
  detail: string;
  confirmText: string;
  cancelText: string;
  type: ModalType;
  resolve: ((value: boolean) => void) | null;
}

interface LoadingState {
  show: boolean;
  title: string;
  message: string;
  cancelable: boolean;
  onCancel: (() => void) | null;
}

interface AlertState {
  show: boolean;
  title: string;
  message: string;
  detail: string;
  confirmText: string;
  type: ModalType;
  resolve: (() => void) | null;
}

class ModalStore {
  confirmState = $state<ConfirmState>({
    show: false,
    title: '',
    message: '',
    detail: '',
    confirmText: '',
    cancelText: '',
    type: 'warning',
    resolve: null,
  });

  loadingState = $state<LoadingState>({
    show: false,
    title: '',
    message: '',
    cancelable: false,
    onCancel: null,
  });

  alertState = $state<AlertState>({
    show: false,
    title: '',
    message: '',
    detail: '',
    confirmText: '',
    type: 'info',
    resolve: null,
  });

  showConfirm = (options: ConfirmOptions): Promise<boolean> => {
    return new Promise((resolve) => {
      this.confirmState = {
        show: true,
        title: options.title || '',
        message: options.message,
        detail: options.detail || '',
        confirmText: options.confirmText || '',
        cancelText: options.cancelText || '',
        type: options.type || 'warning',
        resolve,
      };
    });
  };

  resolveConfirm = (result: boolean) => {
    if (this.confirmState.resolve) {
      this.confirmState.resolve(result);
    }
    this.confirmState.show = false;
    this.confirmState.resolve = null;
  };

  showLoading = (options?: LoadingOptions) => {
    this.loadingState = {
      show: true,
      title: options?.title || '',
      message: options?.message || '',
      cancelable: options?.cancelable || false,
      onCancel: options?.onCancel || null,
    };
  };

  updateLoading = (options: Partial<LoadingOptions>) => {
    if (!this.loadingState.show) return;
    this.loadingState = {
      ...this.loadingState,
      ...options,
    };
  };

  hideLoading = () => {
    this.loadingState.show = false;
    this.loadingState.onCancel = null;
  };

  withLoading = async <T>(fn: () => Promise<T>, options?: LoadingOptions): Promise<T> => {
    this.showLoading(options);
    try {
      return await fn();
    } finally {
      this.hideLoading();
    }
  };

  showAlert = (options: AlertOptions): Promise<void> => {
    return new Promise((resolve) => {
      this.alertState = {
        show: true,
        title: options.title || '',
        message: options.message,
        detail: options.detail || '',
        confirmText: options.confirmText || '',
        type: options.type || 'info',
        resolve,
      };
    });
  };

  resolveAlert = () => {
    if (this.alertState.resolve) {
      this.alertState.resolve();
    }
    this.alertState.show = false;
    this.alertState.resolve = null;
  };
}

export const modalStore = new ModalStore();

export const showConfirm = modalStore.showConfirm;
export const showLoading = modalStore.showLoading;
export const hideLoading = modalStore.hideLoading;
export const withLoading = modalStore.withLoading;
export const showAlert = modalStore.showAlert;
