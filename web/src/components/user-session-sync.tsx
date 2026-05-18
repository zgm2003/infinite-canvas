"use client";

import { useEffect } from "react";

import { useUserStore } from "@/stores/use-user-store";

export function UserSessionSync() {
  const hydrateUser = useUserStore((state) => state.hydrateUser);

  useEffect(() => {
    void hydrateUser();
  }, [hydrateUser]);

  return null;
}
