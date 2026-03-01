const users = []

function addUser(name, email) {
  if (!name || !email) throw new Error("name and email required")
  users.push({ id: users.length + 1, name, email, createdAt: new Date() })
}

function removeUser(id) {
  const index = users.findIndex(u => u.id === id)
  if (index === -1) throw new Error("user not found")
  users.splice(index, 1)
}

function getUserByEmail(email) {
  return users.find(u => u.email === email) || null
}

addUser("alice", "alice@example.com")
addUser("bob", "bob@example.com")
removeUser(1)
console.log(getUserByEmail("bob@example.com"))