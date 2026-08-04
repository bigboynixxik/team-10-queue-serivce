import { create as actualCreate, type StateCreator } from "zustand";

// https://zustand.docs.pmnd.rs/guides/how-to-reset-state

export const storeResetFns = new Set<() => void>();

export const resetAllStores = () => {
	storeResetFns.forEach((resetFn) => {
		resetFn();
	});
};

export const create = (<T>() => {
	return (stateCreator: StateCreator<T>) => {
		const store = actualCreate(stateCreator);
		storeResetFns.add(() => {
			store.setState(store.getInitialState(), true);
		});
		return store;
	};
}) as typeof actualCreate;
