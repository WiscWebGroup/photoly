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

interface NameAndAvatarDrawerProps {
    isOpen: boolean,
    onClose: () => void
}

const NameAndAvatarDrawer: React.FC<NameAndAvatarDrawerProps> = ({isOpen, onClose}) => {
    const [file, setFile] = useState<File>()
    const [username, setUsername] = useState<string>()
    const token = useToken()
    const {post} = useApi()
    const handleFileChange = (e: ChangeEvent<HTMLInputElement>) => {
        if (!!e.target.files) setFile(e.target.files[0])
    }
    const handleUsernameChange = (e: ChangeEvent<HTMLInputElement>) => {
        setUsername(e.target.value)
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
        Router.reload()
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
                <DrawerHeader>Change Username & Avatar</DrawerHeader>
                <DrawerBody>
                    <Input placeholder='Username' onChange={handleUsernameChange}/>
                    <Input type={"file"} mt={4} accept={"image/png, image/jpeg"} onChange={handleFileChange}/>
                </DrawerBody>

                <DrawerFooter>
                    <Button variant='outline' mr={3} onClick={onClose}>
                        Cancel
                    </Button>
                    <Button colorScheme='blue' onClick={() => {
                        handleSubmit();
                        onClose()
                    }}>Save</Button>
                </DrawerFooter>
            </DrawerContent>
        </Drawer>
    )
}

export default NameAndAvatarDrawer