import { SweetAlert } from "./SweetAlert";

export function success(message: string) {
  SweetAlert.success(message);
}

export function failure(message: string) {
  SweetAlert.error(message);
}

export async function confirmAction(title: string, text: string, _danger = false) {
  const result = await SweetAlert.confirm(title, text);
  return result.isConfirmed;
}
