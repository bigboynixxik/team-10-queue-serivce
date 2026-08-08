import { USER_ID_STORAGE_KEY } from "@/shared/config";

export const createUserId = () => {
    const stored = localStorage.getItem(USER_ID_STORAGE_KEY);
    if (stored) return stored;
  
    const userId = crypto.randomUUID();
    localStorage.setItem(USER_ID_STORAGE_KEY, userId);
    return userId;
};