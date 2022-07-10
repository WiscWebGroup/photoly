import {ChangeEvent, useState} from "react";
import {NextRouter, useRouter} from "next/router";
import {useBoolean} from "@chakra-ui/react";

const useLoginForm = (): [
    username: string | undefined,
    email: string | undefined,
    password: string | undefined,
    handleUsernameChange: (e: ChangeEvent<HTMLInputElement>) => void,
    handleEmailChange: (e: ChangeEvent<HTMLInputElement>) => void,
    handlePasswordChange: (e: ChangeEvent<HTMLInputElement>) => void,
    router: NextRouter,
    isLoading: boolean,
    setLoading: { on: () => void, off: () => void, toggle: () => void }
] => {
    const router = useRouter()
    const [username, setUsername] = useState<string | undefined>()
    const [email, setEmail] = useState<string | undefined>()
    const [password, setPassword] = useState<string | undefined>()
    const handleUsernameChange = (event: ChangeEvent<HTMLInputElement>) => setUsername(event.currentTarget.value)
    const handleEmailChange = (event: ChangeEvent<HTMLInputElement>) => setEmail(event.currentTarget.value)
    const handlePasswordChange = (event: ChangeEvent<HTMLInputElement>) => setPassword(event.currentTarget.value)
    const [isLoading, setLoading] = useBoolean(false)
    return [username, email, password, handleUsernameChange, handleEmailChange, handlePasswordChange, router, isLoading, setLoading]
}

export default useLoginForm