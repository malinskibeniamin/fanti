import { useShallow } from 'zustand/react/shallow';

import { Moon, Sun } from '@/components/icons';
import { Button } from '@/components/ui/button';
import { useThemeStore } from '@/stores/theme';

function ThemeToggle() {
  const { theme, toggleTheme } = useThemeStore(
    useShallow((state) => ({
      theme: state.theme,
      toggleTheme: state.toggleTheme,
    })),
  );

  return (
    <Button
      variant="ghost"
      aria-label="Toggle dark mode"
      onClick={toggleTheme}
      className="size-10 rounded-lg text-foreground"
    >
      {theme === 'dark' ? (
        <Sun aria-hidden="true" className="size-5" />
      ) : (
        <Moon aria-hidden="true" className="size-5" />
      )}
    </Button>
  );
}

export { ThemeToggle };
