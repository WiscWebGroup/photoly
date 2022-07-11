import {ChangeEvent, useState***REMOVED*** from "react";
import {NextRouter, useRouter***REMOVED*** from "next/router";

const useLoginForm = (): [
    username: string | undefined,
    email: string | undefined,
    password: string | undefined,
    handleUsernameChange: (e: ChangeEvent<HTMLInputElement>) => void,
    handleEmailChange: (e: ChangeEvent<HTMLInputElement>) => void,
    handlePasswordChange: (e: ChangeEvent<HTMLInputElement>) => void,
    router: NextRouter
] => {
    const router = useRouter()
    const [username, setUsername] = useState<string | undefined>()
    const [email, setEmail] = useState<string | undefined>()
    const [password, setPassword] = useState<string | undefined>()
    const handleUsernameChange = (event: ChangeEvent<HTMLInputElement>) => setUsername(event.currentTarget.value)
    const handleEmailChange = (event: ChangeEvent<HTMLInputElement>) => setEmail(event.currentTarget.value)
    const handlePasswordChange = (event: ChangeEvent<HTMLInputElement>) => setPassword(event.currentTarget.value)
    return [username, email, password, handleUsernameChange, handleEmailChange, handlePasswordChange, router]
***REMOVED***

export default useLoginForm