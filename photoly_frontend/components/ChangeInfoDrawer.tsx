import React, {ChangeEvent, useState} from "react";
import {
    Drawer,
    DrawerBody,
    DrawerFooter,
    DrawerHeader,
    DrawerOverlay,
    DrawerContent,
    DrawerCloseButton,
    Input,
    Button
} from '@chakra-ui/react'
import useApi from "../hooks/useApi";
import useToken from "../hooks/useToken";
import Router from "next/router"

interface ChangeInfoProps {
    isOpen: boolean,
    onClose: () => void
}

const ChangeInfoDrawer: React.FC<ChangeInfoProps> = ({isOpen, onClose}) => {
    const [file, setFile] = useState<File>()
    const [username, setUsername] = useState<string>()
    const [email, setEmail] = useState<string>()
    const token = useToken()
    const {post, isLoading} = useApi()
    const handleFileChange = (e: ChangeEvent<HTMLInputElement>) => {
        if (!!e.target.files) setFile(e.target.files[0])
    }
    const handleUsernameChange = (e: ChangeEvent<HTMLInputElement>) => {
        setUsername(e.target.value)
    }
    const handleEmailChange = (e: ChangeEvent<HTMLInputElement>) => {
        setEmail(e.target.value)
    }
    const handleSubmit = async () => {
        if (!!file){
            await post("/user/updateAvatar", {
                file: file
            }, {
                headers: {
                    "HRD-token": token,
                    "Content-type": "multipart/form-data"
                }
            })
        }
        if (!!username) {
            await post(`/user/updateUsername?username=${username}`, {}, {
                headers: {
                    "HRD-token": token
                }
            })
        }
        if (!!email) {
            await post(`/user/updateEmail?email=${email}`, {}, {
                headers: {
                    "HRD-token": token
                }
            })
        }
        if (!!email || !!username || !!file) Router.reload()
    }

    return (
        <Drawer
            isOpen={isOpen}
            placement='right'
            onClose={onClose}
        >
            <DrawerOverlay/>
            <DrawerContent>
                <DrawerCloseButton/>
                <DrawerHeader>Change Info</DrawerHeader>
                <DrawerBody>
                    <Input placeholder='Username' onChange={handleUsernameChange}/>
                    <Input placeholder='Email' mt={4} onChange={handleEmailChange}/>
                    <Input type={"file"} mt={4} accept={"image/png, image/jpeg"} onChange={handleFileChange}/>
                </DrawerBody>

                <DrawerFooter>
                    <Button variant='outline' mr={3} onClick={onClose}>
                        Cancel
                    </Button>
                    <Button colorScheme='blue' onClick={() => {
                        handleSubmit();
                        onClose()
                    }} isLoading={isLoading} loadingText={"Saving..."}>Save</Button>
                </DrawerFooter>
            </DrawerContent>
        </Drawer>
    )
}

export default ChangeInfoDrawer