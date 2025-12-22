# 🚀 Project Spec: Go-Redis Interactive Explorer

### 1. The Vision
To build a terminal-based user interface (TUI) that acts as a visual dashboard for Redis. Instead of memorizing commands, users interact through menus, forms, and navigable lists to perform CRUD operations and explore the database structure.

### 2. Functional Requirements

#### A. Interactive Operations (The "Wizard" Mode)
* **Set Key:** Select "Set" $\rightarrow$ Input Key $\rightarrow$ Input Value $\rightarrow$ Confirmation.
* **Get Key:** Select "Get" $\rightarrow$ Input Key $\rightarrow$ Display Result.
* **HSet (Hash Set):** Select "HSet" $\rightarrow$ Select/Input Hash Name $\rightarrow$ Input Field Key $\rightarrow$ Input Value $\rightarrow$ Confirmation.

#### B. The Database Explorer (The "Browser" Mode)
* **List Navigation:** View all keys (or hashes) in a scrollable list.
* **Drill Down:** Select a specific Hash/List/Set to see its internal items.
* **Action Menu:** Upon selecting an item, show context-aware options:
    * *Edit Value*
    * *Delete Key*
    * *Copy Value*

### 3. User Flow Diagrams

**Flow 1: The "Set" Operation**
```text
[ Main Menu ]
      │
      └── User selects "SET"
              │
         [ Input Form: Key ]
              │ (User types "myConfig")
              ▼
         [ Input Form: Value ]
              │ (User types "true")
              ▼
       [ Confirmation / Result ]
       "Successfully set myConfig = true"
              │
      (Press Enter to return to Menu)
```

**Flow 2: The "Explorer" Operation**
```text
[ Main Menu ]
      │
      └── User selects "EXPLORE"
              │
        [ List of Keys ]
        │  > user:100
        │  > config:app
        │  > session:xyz
              │ (User selects "user:100")
              ▼
        [ Key Detail View ]
        Type: Hash
        1) name: "Alice"
        2) role: "Admin"
              │ (User hits 'E' to Edit)
              ▼
        [ Update Value Form ]
```

### 4. Technical Architecture

We will move from a simple Request-Response loop to a **State Machine Architecture**.

#### The State Model
The application will always be in one of these discrete states:

| State | Description |
| :--- | :--- |
| **`StateMenu`** | The main landing screen (list of operations). |
| **`StateInputKey`** | The user is currently typing a Key name. |
| **`StateInputValue`** | The user is currently typing a Value. |
| **`StateBrowser`** | The user is scrolling through a list of keys. |
| **`StateLoading`** | Waiting for network response (prevents UI freezing). |

#### The Data Model (Struct)
Your central data structure will expand to hold "Form Data" as the user moves through the wizard steps.

```go
type Model struct {
    // Application State
    CurrentState AppState
    
    // UI Components
    List         list.Model   // For menus and browsing keys
    Input        textinput.Model // For typing keys/values
    Viewport     viewport.Model // For reading long values

    // Form Data (Temporary storage for the wizard)
    ActiveKey    string
    ActiveValue  string
    SelectedOp   string // "SET", "GET", "HSET", etc.
    
    // Backend
    Conn         net.Conn
    Reader       *bufio.Reader
}
```

### 5. Implementation Roadmap

We will build this in layers:

* **Phase 1: The Menu System**
    * Implement `bubbles/list` to show the Main Menu options.
    * Set up the State Machine enum.
* **Phase 2: The Input Wizard**
    * Implement `bubbles/textinput` to capture Keys and Values.
    * Connect the "Set" flow (Menu $\rightarrow$ Key $\rightarrow$ Value $\rightarrow$ Redis).
* **Phase 3: The Explorer**
    * Implement `KEYS *` fetching (using our recursive parser).
    * Populate the list component with real data from Redis.
* **Phase 4: Context Actions**
    * Add "Delete" and "Update" functionality to the Explorer items.