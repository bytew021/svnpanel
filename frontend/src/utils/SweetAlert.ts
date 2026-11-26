import VxeUI from "vxe-pc-ui";
import Swal, { type SweetAlertOptions, type SweetAlertResult } from "sweetalert2";
import "sweetalert2/dist/sweetalert2.css";

interface MessageOptions {
  duration?: number;
  showClose?: boolean;
}

class SweetAlertService {
  success(text: string, options?: MessageOptions) {
    void VxeUI.modal.message({
      content: text,
      status: "success",
      duration: options?.duration ?? 3000,
      showClose: options?.showClose ?? false,
    });
  }

  error(text: string, options?: MessageOptions) {
    void VxeUI.modal.message({
      content: text,
      status: "error",
      duration: options?.duration ?? 3000,
      showClose: options?.showClose ?? false,
    });
  }

  warning(text: string, options?: MessageOptions) {
    void VxeUI.modal.message({
      content: text,
      status: "warning",
      duration: options?.duration ?? 3000,
      showClose: options?.showClose ?? false,
    });
  }

  info(text: string, options?: MessageOptions) {
    void VxeUI.modal.message({
      content: text,
      status: "info",
      duration: options?.duration ?? 3000,
      showClose: options?.showClose ?? false,
    });
  }

  alert(text: string): Promise<SweetAlertResult> {
    return new Promise((resolve) => {
      void Swal.fire({
        position: "top",
        width: "600px",
        html: text,
        showCancelButton: false,
        showConfirmButton: false,
        footer: '<div class="swal-alert-footer"><button class="swal-alert-close-button">关闭</button></div>',
        allowOutsideClick: false,
        scrollbarPadding: false,
        heightAuto: false,
        didOpen: () => {
          Swal.getPopup()?.querySelector(".swal-alert-close-button")?.addEventListener("click", () => {
            resolve({ isConfirmed: false, isDenied: false, isDismissed: true, value: "cancel" });
            Swal.close();
          });
        },
      });
    });
  }

  confirm(
    title: string,
    textOrOptions?: string | SweetAlertOptions,
    options?: SweetAlertOptions,
  ): Promise<SweetAlertResult> {
    let html: SweetAlertOptions["html"];
    let finalOptions: SweetAlertOptions = {};

    if (typeof textOrOptions === "string" || textOrOptions === undefined) {
      const text = textOrOptions;
      html =
        text === undefined
          ? title
          : `
          <h3 class="swal-confirm-title">${title}</h3>
          <hr class="swal-confirm-divider">
          ${text || ""}
        `;
      finalOptions = options || {};
    } else {
      finalOptions = textOrOptions;
      html = finalOptions.html || title;
    }

    return Swal.fire({
      position: "top",
      width: "600px",
      title: "",
      html,
      allowOutsideClick: false,
      customClass: {
        popup: "swal-confirm-popup",
        actions: "swal-confirm-actions",
      },
      showCancelButton: true,
      showConfirmButton: true,
      confirmButtonText: "&nbsp;&nbsp;&nbsp;&nbsp;确认&nbsp;&nbsp;&nbsp;&nbsp;",
      cancelButtonText: "&nbsp;&nbsp;&nbsp;&nbsp;取消&nbsp;&nbsp;&nbsp;&nbsp;",
      confirmButtonColor: "#007bff",
      cancelButtonColor: "#8699b2",
      buttonsStyling: true,
      scrollbarPadding: false,
      heightAuto: false,
      ...finalOptions,
    });
  }
}

export const SweetAlert = new SweetAlertService();
